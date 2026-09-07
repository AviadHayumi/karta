// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

//go:build js && wasm

package main

import (
	"encoding/json"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/run-ai/karta/test/types"
)

func marshal(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal %T: %v", v, err)
	}
	return string(data)
}

func TestDecodeDefinition(t *testing.T) {
	definition, err := decodeDefinition(marshal(t, types.ReactorKarta()))
	if err != nil {
		t.Fatalf("decodeDefinition() error = %v", err)
	}
	if definition.Name != "reactor" {
		t.Errorf("expected definition name = %q, got %q", "reactor", definition.Name)
	}
}

func TestDecodeDefinition_InvalidJSON(t *testing.T) {
	if _, err := decodeDefinition("not json"); err == nil {
		t.Fatal("expected an error for malformed definition JSON")
	}
}

func TestDecodeWorkload(t *testing.T) {
	workload, err := decodeWorkload(marshal(t, types.NewReactorObject()))
	if err != nil {
		t.Fatalf("decodeWorkload() error = %v", err)
	}
	if workload.GetKind() == "" {
		t.Error("expected the decoded workload to carry a kind")
	}
}

func TestDecodeWorkload_InvalidJSON(t *testing.T) {
	if _, err := decodeWorkload("not json"); err == nil {
		t.Fatal("expected an error for malformed workload JSON")
	}
}

func TestDecodePods(t *testing.T) {
	pods, err := decodePods(marshal(t, []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}},
	}))
	if err != nil {
		t.Fatalf("decodePods() error = %v", err)
	}
	if len(pods) != 1 || pods[0].Name != "worker-0" {
		t.Errorf("expected a single pod named %q, got %#v", "worker-0", pods)
	}
}

func TestDecodePods_InvalidJSON(t *testing.T) {
	if _, err := decodePods("not json"); err == nil {
		t.Fatal("expected an error for malformed pods JSON")
	}
}

func TestEncodeEnvelope_Success(t *testing.T) {
	env := encodeEnvelope(map[string]string{"hello": "world"}, nil)

	if !env.Get("error").IsNull() {
		t.Fatalf("expected error = null, got %v", env.Get("error"))
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(env.Get("data").String()), &parsed); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if parsed["hello"] != "world" {
		t.Errorf("expected data.hello = %q, got %q", "world", parsed["hello"])
	}
}

func TestEncodeEnvelope_Error(t *testing.T) {
	env := encodeEnvelope(nil, errors.New("boom"))

	if !env.Get("data").IsNull() {
		t.Fatalf("expected data = null, got %v", env.Get("data"))
	}
	if got := env.Get("error").String(); got != "boom" {
		t.Errorf("expected error = %q, got %q", "boom", got)
	}
}

func TestEncodeEnvelope_MarshalError(t *testing.T) {
	// A channel cannot be marshaled to JSON.
	env := encodeEnvelope(make(chan int), nil)

	if !env.Get("data").IsNull() {
		t.Fatalf("expected data = null, got %v", env.Get("data"))
	}
	if env.Get("error").String() == "" {
		t.Fatal("expected a non-empty error for an unmarshalable value")
	}
}
