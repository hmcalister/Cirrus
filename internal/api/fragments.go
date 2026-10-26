package api

// Data rendered into the shared message template fragment.
//
// Kind selects the styling (error or status); Message is the text shown to the user.
type fragmentData struct {
	Kind    string
	Message string
}

func errorFragment(message string) fragmentData {
	return fragmentData{Kind: "error", Message: message}
}
