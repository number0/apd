// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package apd

import (
	"fmt"
	"math"
	"math/big"
	"testing"
	"unsafe"
)

var (
	testCtx = &BaseContext
)

func (d *Decimal) GoString() string {
	return fmt.Sprintf(`{Coeff: %s, Exponent: %d, Negative: %v, Form: %s}`, d.Coeff.String(), d.Exponent, d.Negative, d.Form)
}

// testExponentError skips t if err was caused by an exponent being outside
// of the package's supported exponent range. Since the exponent is so large,
// we don't support those tests yet (i.e., it's an expected failure, so we
// skip it).
func testExponentError(t *testing.T, err error) {
	if err == nil {
		return
	}
	if err.Error() == errExponentOutOfRangeStr {
		t.Skip(err)
	}
}

func newDecimal(t *testing.T, c *Context, s string) *Decimal {
	d, _, err := c.NewFromString(s)
	testExponentError(t, err)
	if err != nil {
		t.Fatalf("%s: %+v", s, err)
	}
	return d
}

func TestUpscale(t *testing.T) {
	tests := []struct {
		x, y *Decimal
		a, b *big.Int
		s    int32
	}{
		{x: New(1, 0), y: New(100, -1), a: big.NewInt(10), b: big.NewInt(100), s: -1},
		{x: New(1, 0), y: New(10, -1), a: big.NewInt(10), b: big.NewInt(10), s: -1},
		{x: New(1, 0), y: New(10, 0), a: big.NewInt(1), b: big.NewInt(10), s: 0},
		{x: New(1, 1), y: New(1, 0), a: big.NewInt(10), b: big.NewInt(1), s: 0},
		{x: New(10, -2), y: New(1, -1), a: big.NewInt(10), b: big.NewInt(10), s: -2},
		{x: New(1, -2), y: New(100, 1), a: big.NewInt(1), b: big.NewInt(100000), s: -2},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s, %s", tc.x, tc.y), func(t *testing.T) {
			a, b, s, err := upscale(tc.x, tc.y)
			if err != nil {
				t.Fatal(err)
			}
			if a.Cmp(tc.a) != 0 {
				t.Errorf("a: expected %s, got %s", tc.a, a)
			}
			if b.Cmp(tc.b) != 0 {
				t.Errorf("b: expected %s, got %s", tc.b, b)
			}
			if s != tc.s {
				t.Errorf("s: expected %d, got %d", tc.s, s)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		x, y string
		r    string
	}{
		{x: "1", y: "10", r: "11"},
		{x: "1", y: "1e1", r: "11"},
		{x: "1e1", y: "1", r: "11"},
		{x: ".1e1", y: "100e-1", r: "11.0"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s, %s", tc.x, tc.y), func(t *testing.T) {
			x := newDecimal(t, testCtx, tc.x)
			y := newDecimal(t, testCtx, tc.y)
			d := new(Decimal)
			_, err := testCtx.Add(d, x, y)
			if err != nil {
				t.Fatal(err)
			}
			s := d.String()
			if s != tc.r {
				t.Fatalf("expected: %s, got: %s", tc.r, s)
			}
		})
	}
}

func TestCmp(t *testing.T) {
	tests := []struct {
		x, y string
		c    int
	}{
		{x: "1", y: "10", c: -1},
		{x: "1", y: "1e1", c: -1},
		{x: "1e1", y: "1", c: 1},
		{x: ".1e1", y: "100e-1", c: -1},

		{x: ".1e1", y: "100e-2", c: 0},
		{x: "1", y: ".1e1", c: 0},
		{x: "1", y: "1", c: 0},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s, %s", tc.x, tc.y), func(t *testing.T) {
			x := newDecimal(t, testCtx, tc.x)
			y := newDecimal(t, testCtx, tc.y)
			c := x.Cmp(y)
			if c != tc.c {
				t.Fatalf("expected: %d, got: %d", tc.c, c)
			}
		})
	}
}

func TestModf(t *testing.T) {
	tests := []struct {
		x string
		i string
		f string
	}{
		{x: "1", i: "1", f: "0"},
		{x: "1.0", i: "1", f: "0.0"},
		{x: "1.0e1", i: "10", f: "0"},
		{x: "1.0e2", i: "1.0E+2", f: "0"},
		{x: "1.0e-1", i: "0", f: "0.10"},
		{x: "1.0e-2", i: "0", f: "0.010"},
		{x: "1.1", i: "1", f: "0.1"},
		{x: "1234.56", i: "1234", f: "0.56"},
		{x: "1234.56e2", i: "123456", f: "0"},
		{x: "1234.56e4", i: "1.23456E+7", f: "0"},
		{x: "1234.56e-2", i: "12", f: "0.3456"},
		{x: "1234.56e-4", i: "0", f: "0.123456"},
		{x: "1234.56e-6", i: "0", f: "0.00123456"},
		{x: "123456e-8", i: "0", f: "0.00123456"},
		{x: ".123456e8", i: "1.23456E+7", f: "0"},

		{x: "-1", i: "-1", f: "-0"},
		{x: "-1.0", i: "-1", f: "-0.0"},
		{x: "-1.0e1", i: "-10", f: "-0"},
		{x: "-1.0e2", i: "-1.0E+2", f: "-0"},
		{x: "-1.0e-1", i: "-0", f: "-0.10"},
		{x: "-1.0e-2", i: "-0", f: "-0.010"},
		{x: "-1.1", i: "-1", f: "-0.1"},
		{x: "-1234.56", i: "-1234", f: "-0.56"},
		{x: "-1234.56e2", i: "-123456", f: "-0"},
		{x: "-1234.56e4", i: "-1.23456E+7", f: "-0"},
		{x: "-1234.56e-2", i: "-12", f: "-0.3456"},
		{x: "-1234.56e-4", i: "-0", f: "-0.123456"},
		{x: "-1234.56e-6", i: "-0", f: "-0.00123456"},
		{x: "-123456e-8", i: "-0", f: "-0.00123456"},
		{x: "-.123456e8", i: "-1.23456E+7", f: "-0"},
	}
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			x := newDecimal(t, testCtx, tc.x)
			integ, frac := new(Decimal), new(Decimal)
			x.Modf(integ, frac)
			if tc.i != integ.String() {
				t.Fatalf("integ: expected: %s, got: %s", tc.i, integ)
			}
			if tc.f != frac.String() {
				t.Fatalf("frac: expected: %s, got: %s", tc.f, frac)
			}
			a := new(Decimal)
			if _, err := testCtx.Add(a, integ, frac); err != nil {
				t.Fatal(err)
			}
			if a.Cmp(x) != 0 {
				t.Fatalf("%s != %s", a, x)
			}
			if integ.Exponent < 0 {
				t.Fatal(integ.Exponent)
			}
			if frac.Exponent > 0 {
				t.Fatal(frac.Exponent)
			}

			integ2, frac2 := new(Decimal), new(Decimal)
			x.Modf(integ2, nil)
			x.Modf(nil, frac2)
			if integ.CmpTotal(integ2) != 0 {
				t.Fatalf("got %s, expected %s", integ2, integ)
			}
			if frac.CmpTotal(frac2) != 0 {
				t.Fatalf("got %s, expected %s", frac2, frac)
			}
		})
	}

	// Ensure we don't panic on both nil.
	a := new(Decimal)
	a.Modf(nil, nil)
}

func TestInt64(t *testing.T) {
	tests := []struct {
		x   string
		i   int64
		err bool
	}{
		{x: "0.12e1", err: true},
		{x: "0.1e1", i: 1},
		{x: "10", i: 10},
		{x: "12.3e3", i: 12300},
		{x: "1e-1", err: true},
		{x: "1e2", i: 100},
		{x: "1", i: 1},
		{x: "NaN", err: true},
		{x: "Inf", err: true},
		{x: "9223372036854775807", i: 9223372036854775807},
		{x: "-9223372036854775808", i: -9223372036854775808},
		{x: "9223372036854775808", err: true},
	}
	for _, tc := range tests {
		t.Run(tc.x, func(t *testing.T) {
			x := newDecimal(t, testCtx, tc.x)
			i, err := x.Int64()
			hasErr := err != nil
			if tc.err != hasErr {
				t.Fatalf("expected error: %v, got error: %v", tc.err, err)
			}
			if hasErr {
				return
			}
			if i != tc.i {
				t.Fatalf("expected: %v, got %v", tc.i, i)
			}
		})
	}
}

func TestQuoErr(t *testing.T) {
	tests := []struct {
		x, y string
		p    uint32
		err  string
	}{
		{x: "1", y: "1", p: 0, err: errZeroPrecisionStr},
		{x: "1", y: "0", p: 1, err: "division by zero"},
	}
	for _, tc := range tests {
		c := testCtx.WithPrecision(tc.p)
		x := newDecimal(t, testCtx, tc.x)
		y := newDecimal(t, testCtx, tc.y)
		d := new(Decimal)
		_, err := c.Quo(d, x, y)
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != tc.err {
			t.Fatalf("expected %s, got %s", tc.err, err)
		}
	}
}

func TestConditionString(t *testing.T) {
	tests := map[Condition]string{
		Overflow:             "overflow",
		Overflow | Underflow: "overflow, underflow",
		Subnormal | Inexact:  "inexact, subnormal",
	}
	for c, s := range tests {
		t.Run(s, func(t *testing.T) {
			cs := c.String()
			if cs != s {
				t.Errorf("expected %s; got %s", s, cs)
			}
		})
	}
}

func TestFloat64(t *testing.T) {
	tests := []float64{
		0,
		1,
		-1,
		math.MaxFloat32,
		math.SmallestNonzeroFloat32,
		math.MaxFloat64,
		math.SmallestNonzeroFloat64,
	}

	for _, tc := range tests {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			d := new(Decimal)
			d.SetFloat64(tc)
			f, err := d.Float64()
			if err != nil {
				t.Fatal(err)
			}
			if tc != f {
				t.Fatalf("expected %v, got %v", tc, f)
			}
		})
	}
}

func TestCeil(t *testing.T) {
	tests := map[float64]int64{
		0:    0,
		-0.1: 0,
		0.1:  1,
		-0.9: 0,
		0.9:  1,
		-1:   -1,
		1:    1,
		-1.1: -1,
		1.1:  2,
	}

	for f, r := range tests {
		t.Run(fmt.Sprint(f), func(t *testing.T) {
			d, err := new(Decimal).SetFloat64(f)
			if err != nil {
				t.Fatal(err)
			}
			_, err = testCtx.Ceil(d, d)
			if err != nil {
				t.Fatal(err)
			}
			i, err := d.Int64()
			if err != nil {
				t.Fatal(err)
			}
			if i != r {
				t.Fatalf("got %v, expected %v", i, r)
			}
		})
	}
}

func TestFloor(t *testing.T) {
	tests := map[float64]int64{
		0:    0,
		-0.1: -1,
		0.1:  0,
		-0.9: -1,
		0.9:  0,
		-1:   -1,
		1:    1,
		-1.1: -2,
		1.1:  1,
	}

	for f, r := range tests {
		t.Run(fmt.Sprint(f), func(t *testing.T) {
			d, err := new(Decimal).SetFloat64(f)
			if err != nil {
				t.Fatal(err)
			}
			_, err = testCtx.Floor(d, d)
			if err != nil {
				t.Fatal(err)
			}
			i, err := d.Int64()
			if err != nil {
				t.Fatal(err)
			}
			if i != r {
				t.Fatalf("got %v, expected %v", i, r)
			}
		})
	}
}

func TestToStandard(t *testing.T) {
	tests := map[string]string{
		"0":           "0",
		"0.0":         "0.0",
		"0E2":         "000",
		"0E-2":        "0.00",
		"123.456E10":  "1234560000000",
		".9":          "0.9",
		"-.9":         "-0.9",
		"-123.456E10": "-1234560000000",
		"-0E-2":       "-0.00",
	}

	for c, r := range tests {
		t.Run(c, func(t *testing.T) {
			d, _, err := NewFromString(c)
			if err != nil {
				t.Fatal(err)
			}
			s := d.ToStandard()
			if s != r {
				t.Fatalf("got %v, expected %v", s, r)
			}
		})
	}
}

func TestContextSetStringt(t *testing.T) {
	tests := []struct {
		s      string
		c      *Context
		expect string
	}{
		{
			s:      "1.234",
			c:      &BaseContext,
			expect: "1.234",
		},
		{
			s:      "1.234",
			c:      BaseContext.WithPrecision(2),
			expect: "1.2",
		},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d: %s", i, tc.s), func(t *testing.T) {
			d := new(Decimal)
			if _, _, err := tc.c.SetString(d, tc.s); err != nil {
				t.Fatal(err)
			}
			got := d.String()
			if got != tc.expect {
				t.Fatalf("expected: %s, got: %s", tc.expect, got)
			}
		})
	}
}

func TestQuantize(t *testing.T) {
	tests := []struct {
		s      string
		e      int32
		expect string
	}{
		{
			s:      "1.00",
			e:      -1,
			expect: "1.0",
		},
		{
			s:      "2.0",
			e:      -1,
			expect: "2.0",
		},
		{
			s:      "3",
			e:      -1,
			expect: "3.0",
		},
		{
			s:      "9.9999",
			e:      -2,
			expect: "10.00",
		},
	}
	c := BaseContext.WithPrecision(10)
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s: %d", tc.s, tc.e), func(t *testing.T) {
			d, _, err := NewFromString(tc.s)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.Quantize(d, d, tc.e); err != nil {
				t.Fatal(err)
			}
			s := d.String()
			if s != tc.expect {
				t.Fatalf("expected: %s, got: %s", tc.expect, s)
			}
		})
	}
}

func TestCmpOrder(t *testing.T) {
	tests := []struct {
		s     string
		order int
	}{
		{s: "-NaN", order: -4},
		{s: "-sNaN", order: -3},
		{s: "-Infinity", order: -2},
		{s: "-127", order: -1},
		{s: "-1.00", order: -1},
		{s: "-1", order: -1},
		{s: "-0.000", order: -1},
		{s: "-0", order: -1},
		{s: "0", order: 1},
		{s: "1.2300", order: 1},
		{s: "1.23", order: 1},
		{s: "1E+9", order: 1},
		{s: "Infinity", order: 2},
		{s: "sNaN", order: 3},
		{s: "NaN", order: 4},
	}

	for _, tc := range tests {
		t.Run(tc.s, func(t *testing.T) {
			d, _, err := NewFromString(tc.s)
			if err != nil {
				t.Fatal(err)
			}
			o := d.cmpOrder()
			if o != tc.order {
				t.Fatalf("got %d, expected %d", o, tc.order)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		s    string
		zero bool
	}{
		{s: "-NaN", zero: false},
		{s: "-sNaN", zero: false},
		{s: "-Infinity", zero: false},
		{s: "-127", zero: false},
		{s: "-1.00", zero: false},
		{s: "-1", zero: false},
		{s: "-0.000", zero: true},
		{s: "-0", zero: true},
		{s: "0", zero: true},
		{s: "1.2300", zero: false},
		{s: "1.23", zero: false},
		{s: "1E+9", zero: false},
		{s: "Infinity", zero: false},
		{s: "sNaN", zero: false},
		{s: "NaN", zero: false},
	}

	for _, tc := range tests {
		t.Run(tc.s, func(t *testing.T) {
			d, _, err := NewFromString(tc.s)
			if err != nil {
				t.Fatal(err)
			}
			z := d.IsZero()
			if z != tc.zero {
				t.Fatalf("got %v, expected %v", z, tc.zero)
			}
		})
	}
}

func TestReduce(t *testing.T) {
	tests := map[string]int{
		"-0":        0,
		"0":         0,
		"0.0":       0,
		"00":        0,
		"0.00":      0,
		"-01000":    3,
		"01000":     3,
		"-1":        0,
		"1":         0,
		"-10.000E4": 4,
		"10.000E4":  4,
		"-10.00":    3,
		"10.00":     3,
		"-10":       1,
		"10":        1,
		"-143200000000000000000000000000000000000000000000000000000000": 56,
		"143200000000000000000000000000000000000000000000000000000000":  56,
		"Inf": 0,
		"NaN": 0,
	}

	for s, n := range tests {
		t.Run(s, func(t *testing.T) {
			d, _, err := NewFromString(s)
			if err != nil {
				t.Fatal(err)
			}
			_, got := d.Reduce(d)
			if n != got {
				t.Fatalf("got %v, expected %v", got, n)
			}
		})
	}
}

// TestSizeof is meant to catch changes that unexpectedly increase
// the size of the Decimal struct.
func TestSizeof(t *testing.T) {
	var d Decimal
	if s := unsafe.Sizeof(d); s != 48 {
		t.Errorf("sizeof(Decimal) changed: %d", s)
	}
	var c Context
	if s := unsafe.Sizeof(c); s != 32 {
		t.Errorf("sizeof(Context) changed: %d", s)
	}
}
