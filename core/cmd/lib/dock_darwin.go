package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include "pb_abi.h"

static inline void call_set_dock_color(const PBEngineCalls* c, int32_t rgb) { c->set_dock_color(rgb); }
*/
import "C"

import (
	"strconv"
	"strings"

	"projectb/timeline"
)

func (e *nativeEngine) SetDockColor(color string) {
	rgb, err := strconv.ParseInt(strings.TrimPrefix(color, "#"), 16, 32)
	if err != nil {
		timeline.Errorf("dock colour %q is not #rrggbb", color)
		return
	}
	C.call_set_dock_color(e.calls, C.int32_t(rgb))
}
