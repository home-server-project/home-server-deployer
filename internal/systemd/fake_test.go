package systemd

import "testing"

func TestValidUnit(t *testing.T){for _,v:=range []string{"x.service","alpha-smoke.service"}{if !validUnit(v){t.Fatal(v)}};for _,v:=range []string{"../../x.service","x.socket","x\n.service"}{if validUnit(v){t.Fatal(v)}}}
