package mcp_test

import (
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

func TestToolNameOf_IsModuleEntityVerb(t *testing.T) {
	cases := []struct {
		action model.Action
		want   mcp.ToolName
	}{
		{model.Read, "booking.reservation.read"},
		{model.Create, "booking.reservation.create"},
		{model.Update, "booking.reservation.update"},
		{model.Delete, "booking.reservation.delete"},
	}
	for _, c := range cases {
		got := mcp.ToolNameOf(router.Operation{Module: "booking", Entity: "reservation", Action: c.action})
		if got != c.want {
			t.Errorf("ToolNameOf(%v) = %q, want %q", c.action, got, c.want)
		}
	}
}
