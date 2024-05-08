/*
 * Copyright 2024 Galactica Network
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package zkregistry

import (
	"sync"
)

type (
	// Mutex is a mutex for each zk certificate registry.
	// It is used to lock the zk certificate registry address for reading or writing.
	Mutex struct {
		mutexes map[RegistryIndex]*sync.RWMutex
		mu      sync.RWMutex
	}

	// MutexView is a view of the mutex for a specific zk certificate registry.
	MutexView struct {
		index RegistryIndex
		mutex *Mutex
	}
)

func NewMutex() *Mutex {
	return &Mutex{
		mutexes: make(map[RegistryIndex]*sync.RWMutex),
	}
}

func (m *Mutex) NewView(registryIndex RegistryIndex) *MutexView {
	return &MutexView{
		index: registryIndex,
		mutex: m,
	}
}

// Lock locks the mutex for the given address.
func (m *Mutex) Lock(index RegistryIndex) {
	m.getOrCreateMutex(index).Lock()
}

// Unlock unlocks the mutex for the given address.
func (m *Mutex) Unlock(index RegistryIndex) {
	m.getOrCreateMutex(index).Unlock()
}

// RLock locks the mutex for reading for the given address.
func (m *Mutex) RLock(index RegistryIndex) {
	m.getOrCreateMutex(index).RLock()
}

// RUnlock unlocks the mutex for reading for the given address.
func (m *Mutex) RUnlock(index RegistryIndex) {
	m.getOrCreateMutex(index).RUnlock()
}

// getOrCreateMutex gets or creates a mutex for the given address.
func (m *Mutex) getOrCreateMutex(address RegistryIndex) *sync.RWMutex {
	m.mu.RLock()
	mutex, ok := m.mutexes[address]
	m.mu.RUnlock()

	if ok {
		return mutex
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// check if another goroutine has created the mutex
	mutex, ok = m.mutexes[address]
	if ok {
		return mutex
	}

	mutex = &sync.RWMutex{}
	m.mutexes[address] = mutex
	return mutex
}

// Lock locks the registry for the given address.
func (v *MutexView) Lock() {
	v.mutex.Lock(v.index)
}

// Unlock unlocks the registry for the given address.
func (v *MutexView) Unlock() {
	v.mutex.Unlock(v.index)
}

// RLock locks the registry for the given address for reading.
func (v *MutexView) RLock() {
	v.mutex.RLock(v.index)
}

// RUnlock unlocks the registry for the given address for reading.
func (v *MutexView) RUnlock() {
	v.mutex.RUnlock(v.index)
}
