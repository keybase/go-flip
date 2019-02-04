package flip

import (
	"crypto/hmac"
	"encoding/hex"
)

func (g GameID) String() string           { return hex.EncodeToString(g) }
func (u UID) String() string              { return hex.EncodeToString(u) }
func (d DeviceID) String() string         { return hex.EncodeToString(d) }
func (g GameID) Eq(h GameID) bool         { return hmac.Equal(g[:], h[:]) }
func (u UID) Eq(v UID) bool               { return hmac.Equal(u[:], v[:]) }
func (d DeviceID) Eq(e DeviceID) bool     { return hmac.Equal(d[:], e[:]) }
func (u UserDevice) Eq(v UserDevice) bool { return u.U.Eq(v.U) && u.D.Eq(v.D) }
