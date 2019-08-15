package server

// etAddress will return a viable address to connect to
// Currently it is hardcoded to be one neo node until address manager is implemented
func (s *Server) getAddress() (string, error) {
	return "127.0.0.1:20336", nil
}
