// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/go-units"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// ImageLoader is an interface for testing.
type ImageLoader interface {
	ImageLoad(context.Context, io.Reader, bool) (types.ImageLoadResponse, error)
	ImageTag(context.Context, string, string) error
}

// GetImageLoader is a variable so we can override in tests.
var GetImageLoader = func() (ImageLoader, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	cli.NegotiateAPIVersion(context.Background())
	return cli, nil
}

// Tag adds a tag to an already existent image.
func Tag(src, dest name.Tag) error {
	cli, err := GetImageLoader()
	if err != nil {
		return err
	}

	return cli.ImageTag(context.Background(), src.String(), dest.String())
}

// Write saves the image into the daemon as the given tag.
func Write(tag name.Tag, img v1.Image) (string, error) {
	filter, err := probeIncremental(tag, img)
	if err != nil {
		logs.Warn.Printf("Determining incremental load: %v", err)
		return write(tag, img, keepLayers)
	}
	return write(tag, img, filter)
}

func write(tag name.Tag, img v1.Image, lf tarball.LayerFilter) (string, error) {
	cli, err := GetImageLoader()
	if err != nil {
		return "", err
	}

	pr, pw := io.Pipe()
	go func() {
		pw.CloseWithError(tarball.Write(tag, img, pw, tarball.WithLayerFilter(lf)))
	}()

	// write the image in docker save format first, then load it
	resp, err := cli.ImageLoad(context.Background(), pr, false)
	if err != nil {
		return "", fmt.Errorf("loading image: %v", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	r := io.TeeReader(resp.Body, &buf)

	var displayErr error

	// Let's try to parse this thing as a structured response.
	if resp.JSON {
		decoder := json.NewDecoder(r)
		for {
			var msg jsonmessage.JSONMessage
			if err := decoder.Decode(&msg); err == io.EOF {
				break
			} else if err != nil {
				return buf.String(), fmt.Errorf("reading load response body: %v", err)
			}
			displayErr = display(msg)
		}
	}

	// Copy the rest of the response.
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return buf.String(), err
	}

	return buf.String(), displayErr
}

func display(msg jsonmessage.JSONMessage) error {
	if msg.Error != nil {
		return msg.Error
	}
	if msg.Progress != nil {
		logs.Progress.Printf("%s %s", msg.Status, msg.Progress)
	} else if msg.Stream != "" {
		logs.Progress.Print(msg.Stream)
	} else {
		logs.Progress.Print(msg.Status)
	}
	return nil
}

func discardLayers(v1.Layer) (bool, error) {
	return false, nil
}

func keepLayers(v1.Layer) (bool, error) {
	return true, nil
}

func probeIncremental(tag name.Tag, img v1.Image) (tarball.LayerFilter, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}

	// Set<DiffID>
	have := make(map[v1.Hash]struct{})

	probe := empty.Image
	for i := 0; i < len(layers); i++ {
		// Image with first i layers.
		probe, err = mutate.AppendLayers(empty.Image, layers[0])
		if err != nil {
			return nil, err
		}

		probeTag, err := name.NewTag(fmt.Sprintf("%s:%s-layer_%d_probe", tag.Context(), tag.Identifier(), i))
		if err != nil {
			return nil, err
		}

		if _, err := write(probeTag, probe, discardLayers); err != nil {
			return func(layer v1.Layer) (bool, error) {
				diffid, err := layers[i].DiffID()
				if err != nil {
					return true, err
				}

				if _, ok := have[diffid]; ok {
					return false, nil
				}

				return true, nil
			}, nil
		}

		// We don't need to include this layer in the tarball.
		diffid, err := layers[i].DiffID()
		if err != nil {
			return nil, err
		}
		have[diffid] = struct{}{}
	}

	return discardLayers, nil
}

// JSONMessage defines a message struct. It describes
// the created time, where it from, status, ID of the
// message. It's used for docker events.
//
// Inlined from github.com/docker/docker/pkg/jsonmessage to avoid pulling in
// a ton of dependencies.
type JSONMessage struct {
	Stream          string        `json:"stream,omitempty"`
	Status          string        `json:"status,omitempty"`
	Progress        *JSONProgress `json:"progressDetail,omitempty"`
	ProgressMessage string        `json:"progress,omitempty"` //deprecated
	ID              string        `json:"id,omitempty"`
	From            string        `json:"from,omitempty"`
	Time            int64         `json:"time,omitempty"`
	TimeNano        int64         `json:"timeNano,omitempty"`
	Error           *JSONError    `json:"errorDetail,omitempty"`
	ErrorMessage    string        `json:"error,omitempty"` //deprecated
	// Aux contains out-of-band data, such as digests for push signing and image id after building.
	Aux *json.RawMessage `json:"aux,omitempty"`
}

// JSONProgress describes a Progress. terminalFd is the fd of the current terminal,
// Start is the initial value for the operation. Current is the current status and
// value of the progress made towards Total. Total is the end value describing when
// we made 100% progress for an operation.
type JSONProgress struct {
	terminalFd uintptr
	Current    int64 `json:"current,omitempty"`
	Total      int64 `json:"total,omitempty"`
	Start      int64 `json:"start,omitempty"`
	// If true, don't show xB/yB
	HideCounts bool   `json:"hidecounts,omitempty"`
	Units      string `json:"units,omitempty"`
	nowFunc    func() time.Time
	winSize    int
}

func (p *JSONProgress) String() string {
	var (
		width       = p.width()
		pbBox       string
		numbersBox  string
		timeLeftBox string
	)
	if p.Current <= 0 && p.Total <= 0 {
		return ""
	}
	if p.Total <= 0 {
		switch p.Units {
		case "":
			current := units.HumanSize(float64(p.Current))
			return fmt.Sprintf("%8v", current)
		default:
			return fmt.Sprintf("%d %s", p.Current, p.Units)
		}
	}

	percentage := int(float64(p.Current)/float64(p.Total)*100) / 2
	if percentage > 50 {
		percentage = 50
	}
	if width > 110 {
		// this number can't be negative gh#7136
		numSpaces := 0
		if 50-percentage > 0 {
			numSpaces = 50 - percentage
		}
		pbBox = fmt.Sprintf("[%s>%s] ", strings.Repeat("=", percentage), strings.Repeat(" ", numSpaces))
	}

	switch {
	case p.HideCounts:
	case p.Units == "": // no units, use bytes
		current := units.HumanSize(float64(p.Current))
		total := units.HumanSize(float64(p.Total))

		numbersBox = fmt.Sprintf("%8v/%v", current, total)

		if p.Current > p.Total {
			// remove total display if the reported current is wonky.
			numbersBox = fmt.Sprintf("%8v", current)
		}
	default:
		numbersBox = fmt.Sprintf("%d/%d %s", p.Current, p.Total, p.Units)

		if p.Current > p.Total {
			// remove total display if the reported current is wonky.
			numbersBox = fmt.Sprintf("%d %s", p.Current, p.Units)
		}
	}

	if p.Current > 0 && p.Start > 0 && percentage < 50 {
		fromStart := p.now().Sub(time.Unix(p.Start, 0))
		perEntry := fromStart / time.Duration(p.Current)
		left := time.Duration(p.Total-p.Current) * perEntry
		left = (left / time.Second) * time.Second

		if width > 50 {
			timeLeftBox = " " + left.String()
		}
	}
	return pbBox + numbersBox + timeLeftBox
}

// shim for testing
func (p *JSONProgress) now() time.Time {
	if p.nowFunc == nil {
		p.nowFunc = func() time.Time {
			return time.Now().UTC()
		}
	}
	return p.nowFunc()
}

// shim for testing
func (p *JSONProgress) width() int {
	if p.winSize != 0 {
		return p.winSize
	}
	ws, err := term.GetWinsize(p.terminalFd)
	if err == nil {
		return int(ws.Width)
	}
	return 200
}

type JSONError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *JSONError) Error() string {
	return e.Message
}
