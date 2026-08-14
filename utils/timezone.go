package utils

import "time"

const JapanTimeZone = "Asia/Tokyo"

var japanLocation = func() *time.Location {
	loc, err := time.LoadLocation(JapanTimeZone)
	if err != nil {
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}()

func JapanLocation() *time.Location {
	return japanLocation
}

func UseJapanLocalTime() {
	time.Local = JapanLocation()
}
