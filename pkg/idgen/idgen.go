package idgen

var curID int

func Next() int {
	curID++
	return curID
}

func Current() int {
	return curID
}

func SetStartID(val int) {
	curID = val
}
