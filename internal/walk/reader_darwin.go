package walk

func setIOPolicy(gentle bool) {}

func newReader(kind string) reader { return genericReader{} }
