// crctovice.go
// Copyright(c) 2023 Matt Pharr, licensed under the GNU Public License, Version 3.
// SPDX: GPL-3.0-only

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
)

///////////////////////////////////////////////////////////////////////////
// Type definitions for GeoJSON / CRC config parsing

type ARTCC struct {
	VideoMaps []VideoMapSpec `json:"videoMaps"`
}

type VideoMapSpec struct {
	Id        string `json:"id"`                      // corresponds to GeoJSON filename
	Name      string `json:"name"`                    // full name; will use for identification in scenarios
	ShortName string `json:"shortName"`               // for use in DCB menu
	Category  string `json:"starsBrightnessCategory"` // "A" or "B"
	STARSId   int    `json:"starsId"`                 // not yet used
}

type GeoJSON struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type     string `json:"type"`
	Geometry struct {
		Type        string             `json:"type"`
		Coordinates GeoJSONCoordinates `json:"coordinates"`
	} `json:"geometry"`
}

// We only extract lines (at the moment at least) and so we only worry
// about [][2]float32s for coordinates. (For points, this would be
// a single [2]float32 and for polygons, it would be [][][2]float32...)
type GeoJSONCoordinates []Point2LL

func (c *GeoJSONCoordinates) UnmarshalJSON(d []byte) error {
	*c = nil

	var coords []Point2LL
	if err := json.Unmarshal(d, &coords); err == nil {
		*c = coords
	}
	// Don't report any errors but assume that it's a point, polygon, ...
	return nil
}

///////////////////////////////////////////////////////////////////////////

type ViceMapSpec struct {
	Group int    `json:"group"`
	Label string `json:"label"`
	Name  string `json:"name"`
}

///////////////////////////////////////////////////////////////////////////
// Utilities

func errorExit(msg string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func floor(v float32) float32 {
	return float32(math.Floor(float64(v)))
}

func writeJSON(v any, fn string) {
	fmt.Printf("Writing %s... ", fn)
	var w bytes.Buffer
	enc := json.NewEncoder(&w)
	enc.SetIndent("", "    ")
	err := enc.Encode(v)
	errorExit("JSON error", err)
	err = os.WriteFile(fn, w.Bytes(), 0o644)
	errorExit("writing file", err)
	fmt.Printf("Done.\n")
}

type Point2LL [2]float32

func (p Point2LL) MarshalJSON() ([]byte, error) {
	format := func(v float32) string {
		s := fmt.Sprintf("%03d", int(v))
		v -= floor(v)
		v *= 60
		s += fmt.Sprintf(".%02d", int(v))
		v -= floor(v)
		v *= 60
		s += fmt.Sprintf(".%02d", int(v))
		v -= floor(v)
		v *= 1000
		s += fmt.Sprintf(".%03d", int(v))
		return s
	}

	var s string
	if p[1] > 0 {
		s = "\"N"
	} else {
		s = "\"S"
	}
	s += format(abs(p[1]))

	if p[0] > 0 {
		s += ",E"
	} else {
		s += ",W"
	}
	s += format(abs(p[0])) + "\""

	return []byte(s), nil
}

///////////////////////////////////////////////////////////////////////////
// main

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "crctovice: expected ARTCC name as program argument (e.g., ZNY)\n")
		os.Exit(1)
	}
	base := os.Args[1]

	fn := "ARTCCs/" + base + ".json"
	artccFile, err := os.ReadFile(fn)
	errorExit(fmt.Sprintf("%s: unable to read ARTCC definition", fn), err)

	artcc := ARTCC{}
	err = json.Unmarshal(artccFile, &artcc)
	errorExit(fmt.Sprintf("%s: JSON error", artccFile), err)
	fmt.Printf("Read ARTCC definition: %s\n", fn)

	var mapSpecs []ViceMapSpec
	idToName := make(map[string]string)
	for _, m := range artcc.VideoMaps {
		group := 1
		if m.Category == "A" {
			group = 0
		}
		mapSpecs = append(mapSpecs, ViceMapSpec{
			Group: group,
			Label: m.ShortName,
			Name:  m.Name,
		})
		idToName[m.Id] = m.Name
	}

	videoMaps := make(map[string][]Point2LL)

	err = fs.WalkDir(os.DirFS("."), path.Join("VideoMaps", base),
		func(path string, d fs.DirEntry, err error) error {
			errorExit("error walking VideoMaps directory", err)
			fmt.Printf("\rReading " + path + ": ")
			fmt.Printf(".")

			if filepath.Ext(path) != ".geojson" {
				return nil
			}

			if !strings.Contains(path, base) {
				return nil
			}

			fileid, _ := strings.CutSuffix(filepath.Base(path), ".geojson")
			name, ok := idToName[fileid]
			if !ok {
				// This video map isn't used by the ARTCC
				return nil
			}

			file, err := os.ReadFile(path)
			errorExit(fmt.Sprintf("%s: unable to read file", path), err)

			var gj GeoJSON
			err = UnmarshalJSON(file, &gj)
			if err != nil {
				fmt.Printf("\r" + path + ": warning: " + err.Error() + "\n")
			}

			var lines []Point2LL
			for _, f := range gj.Features {
				if f.Type != "Feature" {
					continue
				}

				if f.Geometry.Type != "LineString" {
					continue
				}

				c := f.Geometry.Coordinates
				for i := 0; i < len(c)-1; i++ {
					lines = append(lines, c[i], c[i+1])
				}
			}

			if _, ok := videoMaps[name]; ok {
				fmt.Fprintf(os.Stderr, "%s: multiple definitions\n", name)
				// FIXME: append here or?
				// os.Exit(1)
			}
			videoMaps[name] = lines

			return nil
		})
	errorExit("error walking VideoMaps directory", err)
	fmt.Printf("\rRead video maps                                               \n")

	writeJSON(videoMaps, base+"-videomaps.json")
	writeJSON(mapSpecs, base+".info")
}

// Unmarshal the bytes into the given type but go through some efforts to
// return useful error messages when the JSON is invalid...
func UnmarshalJSON[T any](b []byte, out *T) error {
	err := json.Unmarshal(b, out)
	if err == nil {
		return nil
	}

	decodeOffset := func(offset int64) (line, char int) {
		line, char = 1, 1
		for i := 0; i < int(offset) && i < len(b); i++ {
			if b[i] == '\n' {
				line++
				char = 1
			} else {
				char++
			}
		}
		return
	}

	switch jerr := err.(type) {
	case *json.SyntaxError:
		line, char := decodeOffset(jerr.Offset)
		return fmt.Errorf("Error at line %d, character %d: %v", line, char, jerr)

	case *json.UnmarshalTypeError:
		line, char := decodeOffset(jerr.Offset)
		return fmt.Errorf("Error at line %d, character %d: %s value for %s.%s invalid for type %s",
			line, char, jerr.Value, jerr.Struct, jerr.Field, jerr.Type.String())

	default:
		return err
	}
}
