// Code generated by "stringer -type=Kind quota.go"; DO NOT EDIT.

package quota

import "strconv"

const _Kind_name = "ReadWrite"

var _Kind_index = [...]uint8{0, 4, 9}

func (i Kind) String() string {
	if i < 0 || i >= Kind(len(_Kind_index)-1) {
		return "Kind(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Kind_name[_Kind_index[i]:_Kind_index[i+1]]
}
