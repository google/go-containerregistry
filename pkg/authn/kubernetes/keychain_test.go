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

package kubernetes

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "k8s.io/client-go/kubernetes/fake"
)

var dockerSecretTypes = []secretType{
	dockerConfigJSONSecretType,
	dockerCfgSecretType,
}

type secretType struct {
	name    corev1.SecretType
	key     string
	marshal func(t *testing.T, registry string, auth authn.AuthConfig) []byte
}

func (s *secretType) Create(t *testing.T, namespace, name string, registry string, auth authn.AuthConfig) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: s.name,
		Data: map[string][]byte{
			s.key: s.marshal(t, registry, auth),
		},
	}
}

var dockerConfigJSONSecretType = secretType{
	name: corev1.SecretTypeDockerConfigJson,
	key:  corev1.DockerConfigJsonKey,
	marshal: func(t *testing.T, target string, auth authn.AuthConfig) []byte {
		return toJSON(t, dockerConfigJSON{
			Auths: map[string]authn.AuthConfig{target: auth},
		})
	},
}

var dockerCfgSecretType = secretType{
	name: corev1.SecretTypeDockercfg,
	key:  corev1.DockerConfigKey,
	marshal: func(t *testing.T, target string, auth authn.AuthConfig) []byte {
		return toJSON(t, map[string]authn.AuthConfig{target: auth})
	},
}

func TestAnonymousFallback(t *testing.T) {
	client := fakeclient.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	})

	kc, err := New(context.Background(), client, Options{})
	if err != nil {
		t.Errorf("New() = %v", err)
	}

	testResolve(t, kc, registry(t, "fake.registry.io"), authn.Anonymous)
}

func TestAnonymousFallbackNoServiceAccount(t *testing.T) {
	kc, err := New(context.Background(), nil, Options{
		ServiceAccountName: NoServiceAccount,
	})
	if err != nil {
		t.Errorf("New() = %v", err)
	}

	testResolve(t, kc, registry(t, "fake.registry.io"), authn.Anonymous)
}

func TestSecretNotFound(t *testing.T) {
	client := fakeclient.NewSimpleClientset()

	kc, err := New(context.Background(), client, Options{
		ServiceAccountName: NoServiceAccount,
		ImagePullSecrets:   []string{"not-found"},
	})
	if err != nil {
		t.Errorf("New() = %v", err)
	}

	testResolve(t, kc, registry(t, "fake.registry.io"), authn.Anonymous)
}

func TestServiceAccountNotFound(t *testing.T) {
	client := fakeclient.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	})
	kc, err := New(context.Background(), client, Options{
		ServiceAccountName: "not-found",
	})
	if err != nil {
		t.Errorf("New() = %v", err)
	}

	testResolve(t, kc, registry(t, "fake.registry.io"), authn.Anonymous)
}

func TestImagePullSecretAttachedServiceAccount(t *testing.T) {
	username, password := "foo", "bar"
	client := fakeclient.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svcacct",
			Namespace: "ns",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{{
			Name: "secret",
		}},
	},
		dockerCfgSecretType.Create(t, "ns", "secret", "fake.registry.io", authn.AuthConfig{
			Username: username,
			Password: password,
		}),
	)

	kc, err := New(context.Background(), client, Options{
		Namespace:          "ns",
		ServiceAccountName: "svcacct",
	})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	testResolve(t, kc, registry(t, "fake.registry.io"),
		&authn.Basic{Username: username, Password: password})
}

func TestSecretAttachedServiceAccount(t *testing.T) {
	username, password := "foo", "bar"

	cases := []struct {
		name            string
		createSecret    bool
		useMountSecrets bool
		expected        authn.Authenticator
	}{
		{
			name:            "resolved successfully",
			createSecret:    true,
			useMountSecrets: true,
			expected:        &authn.Basic{Username: username, Password: password},
		},
		{
			name:            "missing secret skipped",
			createSecret:    false,
			useMountSecrets: true,
			expected:        &authn.Basic{},
		},
		{
			name:            "skip option",
			createSecret:    true,
			useMountSecrets: false,
			expected:        &authn.Basic{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			objs := []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svcacct",
						Namespace: "ns",
					},
					Secrets: []corev1.ObjectReference{{
						Name: "secret",
					}},
				},
			}
			if c.createSecret {
				objs = append(objs, dockerCfgSecretType.Create(
					t, "ns", "secret", "fake.registry.io", authn.AuthConfig{
						Username: username,
						Password: password,
					}))
			}
			client := fakeclient.NewSimpleClientset(objs...)

			kc, err := New(context.Background(), client, Options{
				Namespace:          "ns",
				ServiceAccountName: "svcacct",
				UseMountSecrets:    c.useMountSecrets,
			})
			if err != nil {
				t.Fatalf("New() = %v", err)
			}

			testResolve(t, kc, registry(t, "fake.registry.io"), c.expected)
		})
	}

}

// Prioritze picking the first secret
func TestSecretPriority(t *testing.T) {
	secrets := []corev1.Secret{
		*dockerCfgSecretType.Create(t, "ns", "secret", "fake.registry.io", authn.AuthConfig{
			Username: "user", Password: "pass",
		}),
		*dockerCfgSecretType.Create(t, "ns", "secret-2", "fake.registry.io", authn.AuthConfig{
			Username: "anotherUser", Password: "anotherPass",
		}),
	}

	kc, err := NewFromPullSecrets(context.Background(), secrets)
	if err != nil {
		t.Fatalf("NewFromPullSecrets() = %v", err)
	}

	expectedAuth := &authn.Basic{Username: "user", Password: "pass"}
	testResolve(t, kc, registry(t, "fake.registry.io"), expectedAuth)
}

func TestResolveTargets(t *testing.T) {
	// Iterate over target types
	targetTypes := []authn.Resource{
		registry(t, "fake.registry.io"),
		repo(t, "fake.registry.io/repo"),
	}

	for _, secretType := range dockerSecretTypes {
		for _, target := range targetTypes {
			// Drop the .
			testName := secretType.key[1:] + "_" + target.String()

			t.Run(testName, func(t *testing.T) {
				auth := authn.AuthConfig{
					Password: fmt.Sprintf("%x", md5.Sum([]byte(t.Name()))),
					Username: "user" + fmt.Sprintf("%x", md5.Sum([]byte(t.Name()))),
				}

				kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{
					*secretType.Create(t, "ns", "secret", target.String(), auth),
				})

				if err != nil {
					t.Fatalf("New() = %v", err)
				}
				authenticator := &authn.Basic{Username: auth.Username, Password: auth.Password}
				testResolve(t, kc, target, authenticator)
			})
		}
	}
}

func TestAuthWithScheme(t *testing.T) {
	auth := authn.AuthConfig{
		Password: "password",
		Username: "username",
	}

	kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{
		*dockerConfigJSONSecretType.Create(t, "ns", "secret", "https://fake.registry.io", auth),
	})

	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	authenticator := &authn.Basic{Username: auth.Username, Password: auth.Password}
	testResolve(t, kc, registry(t, "fake.registry.io"), authenticator)
	testResolve(t, kc, repo(t, "fake.registry.io/repo"), authenticator)
}

func TestAuthWithPorts(t *testing.T) {
	auth := authn.AuthConfig{
		Password: "password",
		Username: "username",
	}

	kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{
		*dockerConfigJSONSecretType.Create(t, "ns", "secret", "fake.registry.io:5000", auth),
	})

	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	authenticator := &authn.Basic{Username: auth.Username, Password: auth.Password}
	testResolve(t, kc, registry(t, "fake.registry.io:5000"), authenticator)
	testResolve(t, kc, repo(t, "fake.registry.io:5000/repo"), authenticator)

	// Non-matching ports should return Anonymous
	testResolve(t, kc, registry(t, "fake.registry.io:1000"), authn.Anonymous)
	testResolve(t, kc, repo(t, "fake.registry.io:1000/repo"), authn.Anonymous)
}

func TestAuthPathMatching(t *testing.T) {
	rootAuth := authn.AuthConfig{Username: "root", Password: "root"}
	nestedAuth := authn.AuthConfig{Username: "nested", Password: "nested"}
	leafAuth := authn.AuthConfig{Username: "leaf", Password: "leaf"}
	partialAuth := authn.AuthConfig{Username: "partial", Password: "partial"}

	kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{
		*dockerConfigJSONSecretType.Create(t, "ns", "secret-1", "fake.registry.io", rootAuth),
		*dockerConfigJSONSecretType.Create(t, "ns", "secret-2", "fake.registry.io/nested", nestedAuth),
		*dockerConfigJSONSecretType.Create(t, "ns", "secret-3", "fake.registry.io/nested/repo", leafAuth),
		*dockerConfigJSONSecretType.Create(t, "ns", "secret-4", "fake.registry.io/par", partialAuth),
	})

	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	testResolve(t, kc, registry(t, "fake.registry.io"), authn.FromConfig(rootAuth))
	testResolve(t, kc, repo(t, "fake.registry.io/nested"), authn.FromConfig(nestedAuth))
	testResolve(t, kc, repo(t, "fake.registry.io/nested/repo"), authn.FromConfig(leafAuth))
	testResolve(t, kc, repo(t, "fake.registry.io/nested/repo/dirt"), authn.FromConfig(leafAuth))
	testResolve(t, kc, repo(t, "fake.registry.io/partial"), authn.FromConfig(partialAuth))
}

func TestAuthHostNameVariations(t *testing.T) {
	rootAuth := authn.AuthConfig{Username: "root", Password: "root"}
	subdomainAuth := authn.AuthConfig{Username: "sub", Password: "sub"}

	kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{
		*dockerConfigJSONSecretType.Create(t, "ns", "secret-1", "fake.registry.io", rootAuth),
		*dockerConfigJSONSecretType.Create(t, "ns", "secret-2", "1.fake.registry.io", subdomainAuth),
	})

	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	testResolve(t, kc, registry(t, "fake.registry.io"), authn.FromConfig(rootAuth))
	testResolve(t, kc, registry(t, "1.fake.registry.io"), authn.FromConfig(subdomainAuth))

	// Unrecognized subdomain uses Anonymous
	testResolve(t, kc, registry(t, "2.fake.registry.io"), authn.Anonymous)
}

func TestAuthSpecialPathsIgnored(t *testing.T) {
	auth := authn.AuthConfig{Username: "root", Password: "root"}
	auth2 := authn.AuthConfig{Username: "root2", Password: "root2"}

	kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{
		// Note the paths need a trailing '/'
		*dockerConfigJSONSecretType.Create(t, "ns", "secret-1", "https://fake.registry.io/v1/", auth),
		*dockerConfigJSONSecretType.Create(t, "ns", "secret-2", "https://fake2.registry.io/v2/", auth2),
	})

	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	testResolve(t, kc, registry(t, "fake.registry.io"), authn.FromConfig(auth))
	testResolve(t, kc, repo(t, "fake.registry.io/repo"), authn.FromConfig(auth))
	testResolve(t, kc, registry(t, "fake2.registry.io"), authn.FromConfig(auth2))
	testResolve(t, kc, repo(t, "fake2.registry.io/repo"), authn.FromConfig(auth2))
}

func TestAuthDockerRegistry(t *testing.T) {
	auth := authn.AuthConfig{Username: "root", Password: "root"}
	kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{
		*dockerConfigJSONSecretType.Create(t, "ns", "secret", "index.docker.io", auth),
	})

	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	testResolve(t, kc, repo(t, "ubuntu"), authn.FromConfig(auth))
	testResolve(t, kc, repo(t, "knative/serving"), authn.FromConfig(auth))
}

func TestAuthWithGlobs(t *testing.T) {
	auth := authn.AuthConfig{Username: "root", Password: "root"}
	kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{
		*dockerConfigJSONSecretType.Create(t, "ns", "secret", "*.registry.io", auth),
	})

	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	testResolve(t, kc, registry(t, "fake.registry.io"), authn.FromConfig(auth))
	testResolve(t, kc, repo(t, "fake.registry.io/repo"), authn.FromConfig(auth))
	testResolve(t, kc, registry(t, "blah.registry.io"), authn.FromConfig(auth))
	testResolve(t, kc, repo(t, "blah.registry.io/repo"), authn.FromConfig(auth))
}

func testResolve(t *testing.T, kc authn.Keychain, target authn.Resource, expectedAuth authn.Authenticator) {
	t.Helper()

	auth, err := kc.Resolve(target)
	if err != nil {
		t.Errorf("Resolve(%v) = %v", target, err)
	}
	got, err := auth.Authorization()
	if err != nil {
		t.Errorf("Authorization() = %v", err)
	}
	want, err := expectedAuth.Authorization()
	if err != nil {
		t.Errorf("Authorization() = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("Resolve() diff (-want, +got)\n", diff)
	}
}

func toJSON(t *testing.T, obj any) []byte {
	t.Helper()

	bites, err := json.Marshal(obj)

	if err != nil {
		t.Fatal("unable to json marshal", err)
	}
	return bites
}

func registry(t *testing.T, registry string) authn.Resource {
	t.Helper()

	reg, err := name.NewRegistry(registry, name.WeakValidation)
	if err != nil {
		t.Fatal("failed to create registry", err)
	}
	return reg
}

func repo(t *testing.T, repository string) authn.Resource {
	t.Helper()

	repo, err := name.NewRepository(repository, name.WeakValidation)
	if err != nil {
		t.Fatal("failed to create repo", err)
	}
	return repo
}

// TestDockerConfigJSON tests using secrets using the .dockerconfigjson form,
// like you might get from running:
// kubectl create secret docker-registry secret -n ns --docker-server="fake.registry.io" --docker-username="foo" --docker-password="bar"
func TestDockerConfigJSON(t *testing.T) {
	username, password := "foo", "bar"
	kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: "ns",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(
				fmt.Sprintf(`{"auths":{"fake.registry.io":{"username":%q,"password":%q,"auth":%q}}}`,
					username, password,
					base64.StdEncoding.EncodeToString([]byte(username+":"+password))),
			),
		},
	}})
	if err != nil {
		t.Fatalf("NewFromPullSecrets() = %v", err)
	}

	reg, err := name.NewRegistry("fake.registry.io", name.WeakValidation)
	if err != nil {
		t.Errorf("NewRegistry() = %v", err)
	}

	auth, err := kc.Resolve(reg)
	if err != nil {
		t.Errorf("Resolve(%v) = %v", reg, err)
	}
	got, err := auth.Authorization()
	if err != nil {
		t.Errorf("Authorization() = %v", err)
	}
	want, err := (&authn.Basic{Username: username, Password: password}).Authorization()
	if err != nil {
		t.Errorf("Authorization() = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve() = %v, want %v", got, want)
	}
}

func TestKubernetesAuth(t *testing.T) {
	// From https://github.com/knative/serving/issues/12761#issuecomment-1097441770
	// All of these should work with K8s' docker auth parsing.
	for k, ss := range map[string][]string{
		"registry.gitlab.com/dprotaso/test/nginx": {
			"registry.gitlab.com",
			"http://registry.gitlab.com",
			"https://registry.gitlab.com",
			"registry.gitlab.com/dprotaso",
			"http://registry.gitlab.com/dprotaso",
			"https://registry.gitlab.com/dprotaso",
			"registry.gitlab.com/dprotaso/test",
			"http://registry.gitlab.com/dprotaso/test",
			"https://registry.gitlab.com/dprotaso/test",
			"registry.gitlab.com/dprotaso/test/nginx",
			"http://registry.gitlab.com/dprotaso/test/nginx",
			"https://registry.gitlab.com/dprotaso/test/nginx",
		},
		"dtestcontainer.azurecr.io/dave/nginx": {
			"dtestcontainer.azurecr.io",
			"http://dtestcontainer.azurecr.io",
			"https://dtestcontainer.azurecr.io",
			"dtestcontainer.azurecr.io/dave",
			"http://dtestcontainer.azurecr.io/dave",
			"https://dtestcontainer.azurecr.io/dave",
			"dtestcontainer.azurecr.io/dave/nginx",
			"http://dtestcontainer.azurecr.io/dave/nginx",
			"https://dtestcontainer.azurecr.io/dave/nginx",
		}} {
		repo, err := name.NewRepository(k)
		if err != nil {
			t.Errorf("parsing %q: %v", k, err)
			continue
		}

		for _, s := range ss {
			t.Run(fmt.Sprintf("%s - %s", k, s), func(t *testing.T) {
				username, password := "foo", "bar"
				kc, err := NewFromPullSecrets(context.Background(), []corev1.Secret{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "ns",
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(
							fmt.Sprintf(`{"auths":{%q:{"username":%q,"password":%q,"auth":%q}}}`,
								s,
								username, password,
								base64.StdEncoding.EncodeToString([]byte(username+":"+password))),
						),
					},
				}})
				if err != nil {
					t.Fatalf("NewFromPullSecrets() = %v", err)
				}
				auth, err := kc.Resolve(repo)
				if err != nil {
					t.Errorf("Resolve(%v) = %v", repo, err)
				}
				got, err := auth.Authorization()
				if err != nil {
					t.Errorf("Authorization() = %v", err)
				}
				want, err := (&authn.Basic{Username: username, Password: password}).Authorization()
				if err != nil {
					t.Errorf("Authorization() = %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Resolve() = %v, want %v", got, want)
				}
			})
		}
	}
}
