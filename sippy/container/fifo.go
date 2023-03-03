// Copyright (c) 2015 Andrii Pylypenko. All rights reserved.
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification,
// are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
// list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation and/or
// other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
// ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
package sippy_container

type FifoNode struct {
    next *FifoNode
    Value interface{}
}

type Fifo interface {
    Put(interface{})
    Get() *FifoNode
    IsEmpty() bool
}

type fifo struct {
    first *FifoNode
    last *FifoNode
}

func NewFifo() (*fifo) {
    return &fifo{ first : nil, last : nil }
}

func (self *fifo) Put(v interface{}) {
    node := &FifoNode{ next : nil, Value : v }
    if self.last != nil {
        self.last.next = node
        self.last = node
    } else {
        self.first = node
        self.last = node
    }
}

func (self *fifo) Get() (*FifoNode) {
    node := self.first
    if node != nil {
        self.first = node.next
        node.next = nil
    }
    if self.first == nil {
        self.last = nil
    }
    return node
}

func (self *fifo) IsEmpty() bool {
    return self.first == nil
}
