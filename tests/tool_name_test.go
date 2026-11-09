package mcp_test

import (
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

func TestToolNameOf_IsModuleEntityVerb(t *testing.T) {
	tests := []struct {
		action model.Action
		want   string
	}{
		{model.Read, "booking.reservation.read"},
		{model.Create, "booking.reservation.create"},
		{model.Update, "booking.reservation.update"},
		{model.Delete, "booking.reservation.delete"},
	}

	for _, tt := range tests {
		got := mcp.ToolNameOf(router.Operation{Module: "booking", Entity: "reservation", Action: tt.action})
		if string(got) != tt.want {
			t.Errorf("ToolNameOf(action=%c) = %q, want %q", tt.action, got, tt.want)
		}
	}
}
