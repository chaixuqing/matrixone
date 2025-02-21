// Copyright 2021 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"fmt"
	"regexp"
	"time"

	"github.com/matrixorigin/matrixone/pkg/errno"
	"github.com/matrixorigin/matrixone/pkg/sql/errors"
)

const (
	daysPer400Years = 365*400 + 97
	daysPer100Years = 365*100 + 24
	daysPer4Years   = 365*4 + 1
)

type Weekday uint8

const (
	Sunday Weekday = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

var startupTime time.Time
var localTZ int64

func init() {
	startupTime = time.Now()
	_, offset := startupTime.Zone()
	localTZ = int64(offset)
}

var (
	errIncorrectDateValue = errors.New(errno.DataException, "Incorrect date value")

	leapYearMonthDays = []uint8{31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	flatYearMonthDays = []uint8{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

	regDate = regexp.MustCompile(`^(?P<year>[0-9]+)[-](?P<month>[0-9]+)[-](?P<day>[0-9]+)$`)
)

const (
	MaxDateYear    = 9999
	MinDateYear    = 0
	MaxMonthInYear = 12
	MinMonthInYear = 1
)

// ParseDate will parse a string to be a Date
// Support Format:
// `yyyy-mm-dd`
// `yyyymmdd`
func ParseDate(s string) (Date, error) {
	var y int32
	var m, d uint8

	if len(s) < 8 {
		return -1, errIncorrectDateValue
	}

	y = int32(s[0]-'0')*1000 + int32(s[1]-'0')*100 + int32(s[2]-'0')*10 + int32(s[3]-'0')
	if s[4] == '-' {
		if len(s) != 10 || s[7] != '-' {
			return -1, errIncorrectDateValue
		}
		m = (s[5]-'0')*10 + (s[6] - '0')
		d = (s[8]-'0')*10 + (s[9] - '0')
	} else {
		if len(s) != 8 {
			return -1, errIncorrectDateValue
		}
		m = (s[4]-'0')*10 + (s[5] - '0')
		d = (s[6]-'0')*10 + (s[7] - '0')
	}

	if validDate(y, m, d) {
		return FromCalendar(y, m, d), nil
	}
	return -1, errIncorrectDateValue
}

func validDate(year int32, month, day uint8) bool {
	if year >= MinDateYear && year <= MaxDateYear {
		if MinMonthInYear <= month && month <= MaxMonthInYear {
			if day > 0 {
				if isLeap(year) {
					return day <= leapYearMonthDays[month-1]
				} else {
					return day <= flatYearMonthDays[month-1]
				}
			}
		}
	}
	return false
}

func (a Date) String() string {
	y, m, d, _ := a.Calendar(true)
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

// Holds number of days since January 1, year 1 in Gregorian calendar
func Today() Date {
	sec := Now().sec()
	return Date((sec + localTZ) / secsPerDay)
}

const dayInfoTableMinYear = 1924
const dayInfoTableMaxYear = 2099
const dayInfoTableYears = dayInfoTableMaxYear - dayInfoTableMinYear + 1
const dayInfoTableSize = dayInfoTableYears*365 + (dayInfoTableMaxYear-dayInfoTableMinYear)/4 + 1
const dayNumOfTableEpoch = 702360 // the day number of "1924-01-01"

type dayInfo struct {
	year  uint16
	month uint8
	week  uint8
}

var dayInfoTable [dayInfoTableSize]dayInfo

// this init function takes a bit of build time
func init() {
	yearNow := uint16(1924)
	i := int32(0)
	for yearIndex := 0; yearIndex < dayInfoTableYears; yearIndex++ {
		if yearIndex%4 == 0 { // this is a leap year
			for j := 0; j < 366; j++ {
				dayInfoTable[i].year = yearNow
				i++
			}
		} else {
			for j := 0; j < 365; j++ {
				dayInfoTable[i].year = yearNow
				i++
			}
		}
		yearNow++
	}
}

// Year takes a date and returns an uint16 number as the year of this date
func (d Date) Year() uint16 {
	dayNum := int32(d)
	insideDayInfoTable := dayNum >= dayNumOfTableEpoch && dayNum < dayNumOfTableEpoch+dayInfoTableSize
	if insideDayInfoTable {
		return dayInfoTable[dayNum-dayNumOfTableEpoch].year
	}
	// Account for 400 year cycles.
	n := d / daysPer400Years
	y := 400 * n
	d -= daysPer400Years * n

	// Cut off 100-year cycles.
	// The last cycle has one extra leap year, so on the last day
	// of that year, day / daysPer100Years will be 4 instead of 3.
	// Cut it back down to 3 by subtracting n>>2.
	n = d / daysPer100Years
	n -= n >> 2
	y += 100 * n
	d -= daysPer100Years * n

	// Cut off 4-year cycles.
	// The last cycle has a missing leap year, which does not
	// affect the computation.
	n = d / daysPer4Years
	y += 4 * n
	d -= daysPer4Years * n

	// Cut off years within a 4-year cycle.
	// The last year is a leap year, so on the last day of that year,
	// day / 365 will be 4 instead of 3. Cut it back down to 3
	// by subtracting n>>2.
	n = d / 365
	n -= n >> 2
	y += n

	year := uint16(y) + 1

	return year
}

func (d Date) Calendar(full bool) (year int32, month, day uint8, yday uint16) {
	// Account for 400 year cycles.
	n := d / daysPer400Years
	y := 400 * n
	d -= daysPer400Years * n

	// Cut off 100-year cycles.
	// The last cycle has one extra leap year, so on the last day
	// of that year, day / daysPer100Years will be 4 instead of 3.
	// Cut it back down to 3 by subtracting n>>2.
	n = d / daysPer100Years
	n -= n >> 2
	y += 100 * n
	d -= daysPer100Years * n

	// Cut off 4-year cycles.
	// The last cycle has a missing leap year, which does not
	// affect the computation.
	n = d / daysPer4Years
	y += 4 * n
	d -= daysPer4Years * n

	// Cut off years within a 4-year cycle.
	// The last year is a leap year, so on the last day of that year,
	// day / 365 will be 4 instead of 3. Cut it back down to 3
	// by subtracting n>>2.
	n = d / 365
	n -= n >> 2
	y += n
	d -= 365 * n

	year = int32(y) + 1
	yday = uint16(d + 1)

	if !full {
		return
	}

	if isLeap(year) {
		// Leap year
		switch {
		case d > 31+29-1:
			// After leap day; pretend it wasn't there.
			d--
		case d == 31+29-1:
			// Leap day.
			month = 2
			day = 29
			return
		}
	}

	// Estimate month on assumption that every month has 31 days.
	// The estimate may be too low by at most one month, so adjust.
	month = uint8(d / 31)
	end := daysBefore[month+1]
	var begin uint16
	if uint16(d) >= end {
		month++
		begin = end
	} else {
		begin = daysBefore[month]
	}

	month++ // because January is 1
	day = uint8(uint16(d) - begin + 1)
	return year, month, day, yday
}

// daysBefore[m] counts the number of days in a non-leap year
// before month m begins. There is an entry for m=12, counting
// the number of days before January of next year (365).

var daysBefore = [...]uint16{
	0,
	31,
	31 + 28,
	31 + 28 + 31,
	31 + 28 + 31 + 30,
	31 + 28 + 31 + 30 + 31,
	31 + 28 + 31 + 30 + 31 + 30,
	31 + 28 + 31 + 30 + 31 + 30 + 31,
	31 + 28 + 31 + 30 + 31 + 30 + 31 + 31,
	31 + 28 + 31 + 30 + 31 + 30 + 31 + 31 + 30,
	31 + 28 + 31 + 30 + 31 + 30 + 31 + 31 + 30 + 31,
	31 + 28 + 31 + 30 + 31 + 30 + 31 + 31 + 30 + 31 + 30,
	31 + 28 + 31 + 30 + 31 + 30 + 31 + 31 + 30 + 31 + 30 + 31,
}

func FromCalendar(year int32, month, day uint8) Date {
	// Compute days since the absolute epoch.
	d := daysSinceEpoch(year - 1)

	// Add in days before this month.
	d += int32(daysBefore[month-1])
	if isLeap(year) && month >= 3 {
		d++ // February 29
	}

	// Add in days before today.
	d += int32(day - 1)

	return Date(d)
}

func daysSinceEpoch(year int32) int32 {
	// Add in days from 400-year cycles.
	n := year / 400
	year -= 400 * n
	d := daysPer400Years * n

	// Add in 100-year cycles.
	n = year / 100
	year -= 100 * n
	d += daysPer100Years * n

	// Add in 4-year cycles.
	n = year / 4
	year -= 4 * n
	d += daysPer4Years * n

	// Add in non-leap years.
	n = year
	d += 365 * n

	return d
}

func (d Date) DayOfWeek() Weekday {
	// January 1, year 1 in Gregorian calendar, was a Monday.
	return Weekday((d + 1) % 7)
}

func (d Date) DayOfYear() uint16 {
	_, _, _, yday := d.Calendar(false)
	return yday
}

func (d Date) WeekOfYear() (year int32, week uint8) {
	// According to the rule that the first calendar week of a calendar year is
	// the week including the first Thursday of that year, and that the last one is
	// the week immediately preceding the first calendar week of the next calendar year.
	// See https://www.iso.org/obp/ui#iso:std:iso:8601:-1:ed-1:v1:en:term:3.1.1.23 for details.

	// weeks start with Monday
	// Monday Tuesday Wednesday Thursday Friday Saturday Sunday
	// 1      2       3         4        5      6        7
	// +3     +2      +1        0        -1     -2       -3
	// the offset to Thursday
	delta := 4 - int32(d.DayOfWeek())
	// handle Sunday
	if delta == 4 {
		delta = -3
	}
	// find the Thursday of the calendar week
	d = Date(int32(d) + delta)
	year, _, _, yday := d.Calendar(false)
	return year, uint8((yday-1)/7 + 1)
}

func isLeap(year int32) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func (d Date) ToTime() Datetime {
	return Datetime(int64(d)*secsPerDay-localTZ) << 20
}

func (d Date) Month() uint8 {
	_, month, _, _ := d.Calendar(true)
	return month
}
