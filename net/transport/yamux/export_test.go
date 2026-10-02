package yamux

// abandoned returns the number of Open helpers left behind by their callers
func (y *yamuxConn) abandoned() int {
	y.backlogMu.Lock()
	defer y.backlogMu.Unlock()
	return y.abandonedOpens
}
