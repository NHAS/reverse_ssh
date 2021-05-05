package table

import (
	"fmt"
	"io"
	"os"
)

type table struct {
	name          string
	rowNames      []string
	values        [][]string
	valueMaxSizes []int
}

func (t *table) updateMaxs(rn ...string) error {
	if len(rn) > len(t.rowNames) {
		return fmt.Errorf("Wrong size guy")
	}

	if t.valueMaxSizes == nil {
		t.valueMaxSizes = make([]int, len(t.rowNames))
	}

	for i, v := range rn {
		if t.valueMaxSizes[i] < len(v) {
			t.valueMaxSizes[i] = len(v)
		}
	}

	return nil
}

func (t *table) addRow(rn ...string) error {
	t.rowNames = append(t.rowNames, rn...)

	err := t.updateMaxs(rn...)
	if err != nil {
		return err
	}

	return nil
}

func (t *table) AddValues(vals ...string) error {
	if len(vals) > len(t.rowNames) {
		return fmt.Errorf("Error more values than exist in the row name")
	}

	t.values = append(t.values, vals)

	err := t.updateMaxs(vals...)
	if err != nil {
		return err
	}

	return nil
}

func (t *table) Print() {
	t.Fprint(os.Stdout)
}

func (t *table) Fprint(w io.Writer) {

	top := "|"
	for i, rh := range t.rowNames {
		top += fmt.Sprintf(" %-"+fmt.Sprintf("%d", t.valueMaxSizes[i])+"s |", rh)
	}

	fmt.Fprintf(w, "%"+fmt.Sprintf("%d", len(top)/2-len(t.name))+"s\n", t.name)

	seperator(w, len(top))
	fmt.Fprintln(w, top)
	seperator(w, len(top))

	for _, row := range t.values {
		line := "|"
		for i, v := range row {
			line += fmt.Sprintf(" %-"+fmt.Sprintf("%d", t.valueMaxSizes[i])+"s |", v)

		}
		fmt.Fprintln(w, line)
		seperator(w, len(line))
	}
}

func seperator(w io.Writer, i int) {
	for n := 0; n < i; n++ {
		fmt.Fprint(w, "-")
	}
	fmt.Fprint(w, "\n")
}

func NewTable(name string, rows ...string) (t table, err error) {

	err = t.addRow(rows...)
	if err != nil {
		return
	}

	t.name = name

	return
}
