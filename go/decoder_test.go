package rod

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/anaminus/deep"
)

var sampleControl = _struct{
	"Instances": _array{
		_struct{
			"Children": _array{
				_struct{
					"Children": _array{
						_int(1),
						_int(2),
						_int(3),
						_int(4),
						_int(5),
						_int(6),
						_int(7),
						_int(8),
					},
					"ClassName":  "Camera",
					"IsService":  false,
					"Properties": _array{},
					"Reference":  _int(1)},
				_struct{
					"Children":  _array{},
					"ClassName": "Terrain",
					"IsService": false,
					"Properties": _array{
						_struct{
							"Name": "MaterialColors",
							"Type": "BinaryString",
							"Value": _blob{
								0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x6a, 0x7f, 0x3f, 0x3f, 0x7f, 0x6b, 0x7f, 0x66, 0x3f, 0x8a,
								0x56, 0x3e, 0x8f, 0x7e, 0x5f, 0x8b, 0x6d, 0x4f, 0x66, 0x6c, 0x6f, 0x65, 0xb0, 0xea, 0xc3, 0xc7,
								0xda, 0x89, 0x5a, 0x47, 0x3a, 0x2e, 0x24, 0x1e, 0x1e, 0x25, 0x66, 0x5c, 0x3b, 0xe8, 0x9c, 0x4a,
								0x73, 0x7b, 0x6b, 0x84, 0x7b, 0x5a, 0x81, 0xc2, 0xe0, 0x73, 0x84, 0x4a, 0xc6, 0xbd, 0xb5, 0xce,
								0xad, 0x94, 0x94, 0x94, 0x8c,
							},
						},
					},
					"Reference": _int(2),
				},
			},
			"ClassName": "Work\"space",
			"IsService": true,
			"Map": _map{
				true:          false,
				_float(-3.14): _struct{},
				"A":           _int(1),
			},
			"Properties": _array{
				_struct{
					"Name":  "AllowThirdPartySales",
					"Type":  "bool",
					"Value": false,
				},
				_struct{
					"Name":  "AttributeSerialize",
					"Type":  "BinaryString",
					"Value": _blob{},
				},
				_struct{
					"Name":  "CurrentCamera",
					"Type":  "Reference",
					"Value": _int(1),
				},
				_struct{
					"Name": "ModelInPrimary",
					"Type": "CFrame",
					"Value": _struct{
						"R00": _float(1),
						"R01": _float(0),
						"R02": _float(0),
						"R10": _float(0),
						"R11": _float(1),
						"R12": _float(0),
						"R20": _float(0),
						"R21": _float(0),
						"R22": _float(1),
						"X":   _float(0),
						"Y":   _float(0),
						"Z":   _float(0),
					},
				},
			},
			"Reference": _int(0),
		},
	},
}

func TestDecoder(t *testing.T) {
	b, err := os.ReadFile("testdata/sample.rod")
	if err != nil {
		t.Fatalf("%s", err)
		return
	}

	d := NewDecoder(bytes.NewReader(b))
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatalf("%s", err)
	}

	if diffs := deep.Equal(v, sampleControl); len(diffs) > 0 {
		for _, d := range diffs {
			t.Log(d)
		}
		t.Errorf("decoded sample file not equal to control")
	}

	for _, file := range keysOf(testPrimitives) {
		t.Run(fmt.Sprintf("%#q", file), func(t *testing.T) {
			result := testPrimitives[file]
			d := NewDecoder(strings.NewReader(file))
			var v any
			err := d.Decode(&v)
			if (err == nil && result.err != nil) || (err != nil && result.err == nil) {
				t.Fatalf("mismatched error: expected [%s], got [%s]", result.err, err)
			}
			if err != nil {
				return
			}
			if diffs := deep.Equal(v, result.v); len(diffs) > 0 {
				for _, d := range diffs {
					t.Log(d)
				}
				t.Errorf("decoded file not equal to control")
			}
		})
	}
}
