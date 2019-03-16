package things

type Shorturl struct {
	Created int64
	Short   string
	Long    string
	Hits    int64
}

func (s *Shorturl) GetType() string {
	return "Shorturls"
}

func (s *Shorturl) Name() string {
	return s.Short
}

func (s *Shorturl) Date() int64 {
	return s.Created
}

func (s *Shorturl) UpdateHits() {
	s.Hits = s.Hits + 1
}
