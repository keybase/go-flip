package flip

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func (g GameID) String() string           { return hex.EncodeToString(g) }
func (u UID) String() string              { return hex.EncodeToString(u) }
func (d DeviceID) String() string         { return hex.EncodeToString(d) }
func (g GameID) Eq(h GameID) bool         { return hmac.Equal(g[:], h[:]) }
func (u UID) Eq(v UID) bool               { return hmac.Equal(u[:], v[:]) }
func (d DeviceID) Eq(e DeviceID) bool     { return hmac.Equal(d[:], e[:]) }
func (u UserDevice) Eq(v UserDevice) bool { return u.U.Eq(v.U) && u.D.Eq(v.D) }

func (t Time) Time() time.Time {
	if t == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64(t)*1000000)
}

func ToTime(t time.Time) Time {
	if t.IsZero() {
		return 0
	}
	return Time(t.UnixNano() / 1000000)
}

func GenerateGameID() GameID {
	l := 12
	ret := make([]byte, l)
	n, err := rand.Read(ret[:])
	if n != l {
		panic("short random read")
	}
	if err != nil {
		panic(fmt.Sprintf("error reading randomness: %s", err.Error()))
	}
	return GameID(ret)
}
