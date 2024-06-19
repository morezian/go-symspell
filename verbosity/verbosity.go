package verbosity

type Verbosity int // Verbosity controls the closeness/quantity of returned spelling suggestions

const (
	Top     Verbosity = iota // Top suggestion with the highest term frequency of the suggestions of smallest edit distance found
	Closest                  // Closest suggestions with the smallest edit distance found, ordered by frequency
	All                      // All suggestions within maxEditDistance, suggestions ordered by edit distance then by frequency (slower, no early termination)
)
