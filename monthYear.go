package AuthorizeNet

type MonthYearType struct {
	Month, Year string
}

func (my MonthYearType) String() string {
	return my.Month + "/" + my.Year
}
