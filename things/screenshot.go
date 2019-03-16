package things

type Screenshot struct {
	Created  int64
	Filename string
	Hits     int64
}

func (s *Screenshot) GetType() string {
	return "Screenshots"
}

func (s *Screenshot) Name() string {
	return s.Filename
}

func (s *Screenshot) Date() int64 {
	return s.Created
}

func (s *Screenshot) UpdateHits() {
	s.Hits = s.Hits + 1
}
