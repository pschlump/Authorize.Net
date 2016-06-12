package AuthorizeNet

type Bool bool

func (v Bool) String() string {
	if v {
		return "true"
	}
	return "false"
}

func (v Bool) UpperString() string {
	if v {
		return "TRUE"
	}
	return "FALSE"
}
