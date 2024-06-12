// crctovice.go
// Copyright(c) 2023 Matt Pharr, licensed under the GNU Public License, Version 3.
// SPDX: GPL-3.0-only

package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"path"
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

// Note: this should match STARSMap in stars.go
type STARSMap struct {
	Group int
	Label string
	Name  string
	Id    int
	Lines [][]Point2LL
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

func write(maps []STARSMap, fn string) {
	// Write the GOB file with everything
	gfn := fn + "-videomaps.gob"
	fmt.Printf("Writing %s... ", gfn)
	gf, err := os.Create(gfn)
	errorExit("creating file", err)
	defer gf.Close()
	err = gob.NewEncoder(gf).Encode(maps)
	errorExit("GOB error", err)

	// Write the manifest file (without the lines)
	names := make(map[string]interface{})
	for _, m := range maps {
		names[m.Name] = nil
	}
	mfn := fn + "-manifest.gob"
	fmt.Printf("Writing %s... ", mfn)
	mf, err := os.Create(mfn)
	errorExit("creating file", err)
	defer mf.Close()
	err = gob.NewEncoder(mf).Encode(names)
	errorExit("GOB error", err)

	fmt.Printf("Done.\n")
}

// MapSlice returns the slice that is the result of applying the provided
// xform function to all of the elements of the given slice.
func MapSlice[F, T any](from []F, xform func(F) T) []T {
	var to []T
	for _, item := range from {
		to = append(to, xform(item))
	}
	return to
}

type Point2LL [2]float32

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

	var maps []STARSMap
	for _, m := range artcc.VideoMaps {
		group := 1
		if m.Category == "A" {
			group = 0
		}
		sm := STARSMap{
			Group: group,
			Label: m.ShortName,
			Name:  m.Name,
			Id:    m.STARSId,
		}

		fn := path.Join("VideoMaps", base, m.Id) + ".geojson"
		file, err := os.ReadFile(fn)
		errorExit(fmt.Sprintf("%s: unable to read file", fn), err)

		var gj GeoJSON
		err = UnmarshalJSON(file, &gj)
		if err != nil {
			fmt.Printf("\r" + fn + ": warning: " + err.Error() + "\n")
		}

		for _, f := range gj.Features {
			if f.Type != "Feature" {
				continue
			}

			if f.Geometry.Type != "LineString" {
				continue
			}

			sm.Lines = append(sm.Lines, f.Geometry.Coordinates)
		}

		maps = append(maps, sm)
	}
	fmt.Printf("\rRead video maps                                               \n")

	write(maps, base)
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
