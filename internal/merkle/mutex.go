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

package merkle

import "sync"

type (
	// TreeMutex is a mutex for each tree address.
	// It is used to lock the tree address for reading or writing.
	TreeMutex struct {
		mutexes map[TreeIndex]*sync.RWMutex
		mu      sync.RWMutex
	}
)

func NewTreeMutex() *TreeMutex {
	return &TreeMutex{
		mutexes: make(map[TreeIndex]*sync.RWMutex),
	}
}

// Lock locks the mutex for the given address.
func (m *TreeMutex) Lock(address TreeIndex) {
	m.getOrCreateMutex(address).Lock()
}

// Unlock unlocks the mutex for the given address.
func (m *TreeMutex) Unlock(address TreeIndex) {
	m.getOrCreateMutex(address).Unlock()
}

// RLock locks the mutex for reading for the given address.
func (m *TreeMutex) RLock(address TreeIndex) {
	m.getOrCreateMutex(address).RLock()
}

// RUnlock unlocks the mutex for reading for the given address.
func (m *TreeMutex) RUnlock(address TreeIndex) {
	m.getOrCreateMutex(address).RUnlock()
}

// getOrCreateMutex gets or creates a mutex for the given address.
func (m *TreeMutex) getOrCreateMutex(address TreeIndex) *sync.RWMutex {
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
