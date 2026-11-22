//go:build !darwin

package clipboard

// System is the clipboard, or nil on a platform where vimango has none.
func System() *Board {
	return nil
}

// Text is the text on the clipboard, and whether there is any.
func (b *Board) Text() (string, bool) {
	return "", false
}

// SetText puts text on the clipboard and reports whether it did.
func (b *Board) SetText(text string) bool {
	return false
}
