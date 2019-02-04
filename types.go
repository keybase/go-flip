package flip

import (
	"encoding/hex"
)

func (g GameID) String() string   { return hex.EncodeToString(g) }
func (u UID) String() string      { return hex.EncodeToString(u) }
func (d DeviceID) String() string { return hex.EncodeToString(d) }
