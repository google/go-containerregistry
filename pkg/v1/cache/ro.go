package cache

import "github.com/google/go-containerregistry/pkg/v1"

// ReadOnly returns a read-only implementation of the given Cache.
//
// Put and Delete operations are a no-op.
func ReadOnly(c Cache) Cache { return &ro{Cache: c} }

type ro struct{ Cache }

func (ro) Put(v1.Layer) error   { return nil }
func (ro) Delete(v1.Hash) error { return nil }
