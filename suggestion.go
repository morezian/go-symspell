package symspell

import "fmt"

// Suggestion contains a spelling suggestion
type Suggestion struct {
	Term      string // Term is the suggested correctly spelled word
	Distance  int    // Distance between searched for word and suggestion in terms of edits
	Frequency int    // Frequency of suggestion in the dictionary (a measure of how common the word is)
}

// NewSuggestion creates a new instance of a Suggestion
func NewSuggestion(term string, dist, freq int) *Suggestion {
	return &Suggestion{Term: term, Distance: dist, Frequency: freq}
}

// Less returns true if other is less. False if equal or greater than
func (s *Suggestion) Less(other *Suggestion) bool {
	if s.Distance == other.Distance {
		return s.Frequency < other.Frequency
	}
	return s.Distance < other.Distance
}

// String implements the stringer interface
func (s *Suggestion) String() string {
	return fmt.Sprintf("{%s, %d, %d}", s.Term, s.Distance, s.Frequency)
}

// GetHashCode does something, need to figure out what exactly
func (s *Suggestion) GetHashCode() int {
	// term.GetHashCode() in original code
	return 0 // TODO: Need ot figure out how this works
}

// ShallowCopy creates a copy of the suggestion
func (s *Suggestion) ShallowCopy() *Suggestion {
	return NewSuggestion(s.Term, s.Distance, s.Frequency)
}

// Suggestions exists to implement the sort interface
type Suggestions []*Suggestion

// NewSuggestions returns an empty Suggestions set
func NewSuggestions() Suggestions {
	return make([]*Suggestion, 0)
}

// Len returns the length of the Suggestions array
func (s Suggestions) Len() int {
	return len(s)
}

// Swap the positions of two Suggestion in the array
func (s Suggestions) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less compares two Suggestion items and returns true if the first
// is less than the second, false for equal or greater
func (s Suggestions) Less(i, j int) bool {
	return s[i].Less(s[j])
}

// Clear the contents of the suggestions list
func (s Suggestions) Clear() {
	s = make([]*Suggestion, 0)
}
