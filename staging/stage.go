package staging

type node[T any] struct {
	value T
	next  int
}

type entry struct {
	count int
	first int
}

type Stage[T any] struct {
	Deletes map[int]entry
	Nodes   []node[T]
}

func NewSuggestionStage[T any](initialCapacity int) *Stage[T] {
	return &Stage[T]{
		Deletes: make(map[int]entry, initialCapacity),
		Nodes:   make([]node[T], initialCapacity),
	}
}

func (s *Stage[T]) DeleteCount() int {
	return len(s.Deletes)
}

func (s *Stage[T]) NodeCount() int {
	return len(s.Nodes)
}

func (s *Stage[T]) Clear() {
	s.Deletes = make(map[int]entry)
	s.Nodes = make([]node[T], 0)
}

func (s *Stage[T]) Add(deleteHash int, value T) {

	e, prs := s.Deletes[deleteHash]
	if !prs {
		e = entry{
			count: 0,
			first: -1,
		}
	}

	next := e.first
	e.count++
	e.first = s.NodeCount()
	s.Deletes[deleteHash] = e
	s.Nodes = append(s.Nodes, node[T]{
		value: value,
		next:  next,
	})
}

func (s *Stage[T]) CommitTo(permanentDeletes map[int][]T) {
	var suggestions []T
	var ok bool
	var i int

	for key, value := range s.Deletes {
		if suggestions, ok = permanentDeletes[key]; ok {
			i = len(suggestions)
			newSuggestions := make([]T, i+value.count)
			newSuggestions = append(suggestions, newSuggestions...)
		} else {
			i = 0
			suggestions = make([]T, value.count)
			permanentDeletes[key] = suggestions
		}
		next := value.first
		for next >= 0 {
			n := s.Nodes[i]
			suggestions[key] = n.value
			next = n.next
			i++
		}
	}
}
