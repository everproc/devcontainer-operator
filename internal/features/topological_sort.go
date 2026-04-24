package features

import (
	"fmt"
	"slices"
)

type stack[T any] struct {
	inner []T
}

func NewStack[T any]() *stack[T] {
	return &stack[T]{
		inner: make([]T, 0),
	}
}

func (s *stack[T]) Len() int {
	return len(s.inner)
}

func (s *stack[T]) PushBack(m T) {
	s.inner = append(s.inner, m)
}

func (s *stack[T]) PushFront(m T) {
	s.inner = prepend(s.inner, m)
}

func (s *stack[T]) PopBack() T {
	m := s.inner[len(s.inner)-1]
	s.inner = s.inner[:len(s.inner)-1]
	return m
}

func (s *stack[T]) PopFront() T {
	m := s.inner[0]
	s.inner = s.inner[1:]
	return m
}

func prepend[T any](slice []T, value T) []T {
	newSlice := make([]T, len(slice)+1)
	newSlice[0] = value
	copy(newSlice[1:], slice)
	return newSlice
}

func topologicalSort(root *Node[*Feature], allNodes []*Node[*Feature]) []*Node[*Feature] {
	lst := []*Node[*Feature]{}
	stc := NewStack[*Node[*Feature]]()
	for _, n := range allNodes {
		if len(n.Leafs) == 0 {
			// these are the starting points
			stc.PushFront(n)
		}
	}
	visited := []string{}
	for stc.Len() > 0 {
		n := stc.PopFront()
		if n == root {
			// TODO(juf): I dont like this implementation I did here
			// Reached root node
			break
		}
		if slices.Contains(visited, n.ID()) {
			panic(fmt.Sprintf("Cycle at: %s\n", n.Data.Name()))
		}
		visited = append(visited, n.ID())
		lst = append(lst, n)
		if len(n.Parent.Leafs) > 0 {
			c := len(n.Parent.Leafs)
			for _, lf := range n.Parent.Leafs {
				if slices.Contains(visited, lf.ID()) {
					c--
				}
			}
			if c == 0 {
				stc.PushBack(n.Parent)
			}
		}
	}
	return lst
}
