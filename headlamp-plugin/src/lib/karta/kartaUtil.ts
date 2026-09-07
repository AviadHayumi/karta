// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

import { Envelope, getKartaWasm } from './karta';
import { Karta, Pod, PodComponentMatch, Workload, WorkloadTree } from './karta.types';

function unwrap<T>(envelope: Envelope, fallback: T): T {
  if (envelope.error !== null) {
    throw new Error(envelope.error);
  }
  if (envelope.data === null) {
    return fallback;
  }
  return JSON.parse(envelope.data) as T;
}

export async function buildTree(definition: Karta, workload: Workload): Promise<WorkloadTree> {
  const karta = await getKartaWasm();
  return unwrap(karta.buildTree(JSON.stringify(definition), JSON.stringify(workload)), {
    Status: null,
    Children: [],
  });
}

export async function inferPodComponents(definition: Karta, workload: Workload, pods: Pod[]): Promise<PodComponentMatch[]> {
  const karta = await getKartaWasm();
  return unwrap(
    karta.inferPodComponents(JSON.stringify(definition), JSON.stringify(workload), JSON.stringify(pods)),
    []
  );
}

export async function evaluatePhases(definition: Karta, workload: Workload): Promise<string[]> {
  const karta = await getKartaWasm();
  return unwrap(karta.evaluatePhases(JSON.stringify(definition), JSON.stringify(workload)), []);
}

export async function listCatalog(): Promise<Karta[]> {
  const karta = await getKartaWasm();
  return unwrap(karta.listCatalog(), []);
}
