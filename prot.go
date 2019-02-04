// Auto-generated by avdl-compiler v1.3.29 (https://github.com/keybase/node-avdl-compiler)
//   Input file: prot.avdl

package flip

import (
	"errors"
	"github.com/keybase/go-framed-msgpack-rpc/rpc"
)

type Time int64

func (o Time) DeepCopy() Time {
	return o
}

type GameID []byte

func (o GameID) DeepCopy() GameID {
	return (func(x []byte) []byte {
		if x == nil {
			return nil
		}
		return append([]byte{}, x...)
	})(o)
}

type UID []byte

func (o UID) DeepCopy() UID {
	return (func(x []byte) []byte {
		if x == nil {
			return nil
		}
		return append([]byte{}, x...)
	})(o)
}

type DeviceID []byte

func (o DeviceID) DeepCopy() DeviceID {
	return (func(x []byte) []byte {
		if x == nil {
			return nil
		}
		return append([]byte{}, x...)
	})(o)
}

type Start struct {
	StartTime         Time           `codec:"startTime" json:"startTime"`
	CommitmentEndTime Time           `codec:"commitmentEndTime" json:"commitmentEndTime"`
	RevealPeriodMsec  int64          `codec:"revealPeriodMsec" json:"revealPeriodMsec"`
	Params            FlipParameters `codec:"params" json:"params"`
}

func (o Start) DeepCopy() Start {
	return Start{
		StartTime:         o.StartTime.DeepCopy(),
		CommitmentEndTime: o.CommitmentEndTime.DeepCopy(),
		RevealPeriodMsec:  o.RevealPeriodMsec,
		Params:            o.Params.DeepCopy(),
	}
}

type UserDevice struct {
	U UID      `codec:"u" json:"u"`
	D DeviceID `codec:"d" json:"d"`
}

func (o UserDevice) DeepCopy() UserDevice {
	return UserDevice{
		U: o.U.DeepCopy(),
		D: o.D.DeepCopy(),
	}
}

type CommitmentComplete struct {
	Players []UserDevice `codec:"players" json:"players"`
}

func (o CommitmentComplete) DeepCopy() CommitmentComplete {
	return CommitmentComplete{
		Players: (func(x []UserDevice) []UserDevice {
			if x == nil {
				return nil
			}
			ret := make([]UserDevice, len(x))
			for i, v := range x {
				vCopy := v.DeepCopy()
				ret[i] = vCopy
			}
			return ret
		})(o.Players),
	}
}

type FlipType int

const (
	FlipType_INTS    FlipType = 1
	FlipType_SHUFFLE FlipType = 2
)

func (o FlipType) DeepCopy() FlipType { return o }

var FlipTypeMap = map[string]FlipType{
	"INTS":    1,
	"SHUFFLE": 2,
}

var FlipTypeRevMap = map[FlipType]string{
	1: "INTS",
	2: "SHUFFLE",
}

func (e FlipType) String() string {
	if v, ok := FlipTypeRevMap[e]; ok {
		return v
	}
	return ""
}

type IntType int

const (
	IntType_FIXED IntType = 1
	IntType_BIG   IntType = 2
	IntType_BOOL  IntType = 3
)

func (o IntType) DeepCopy() IntType { return o }

var IntTypeMap = map[string]IntType{
	"FIXED": 1,
	"BIG":   2,
	"BOOL":  3,
}

var IntTypeRevMap = map[IntType]string{
	1: "FIXED",
	2: "BIG",
	3: "BOOL",
}

func (e IntType) String() string {
	if v, ok := IntTypeRevMap[e]; ok {
		return v
	}
	return ""
}

type FlipParametersInt struct {
	T__     IntType `codec:"t" json:"t"`
	Big__   *[]byte `codec:"big,omitempty" json:"big,omitempty"`
	Fixed__ *int64  `codec:"fixed,omitempty" json:"fixed,omitempty"`
}

func (o *FlipParametersInt) T() (ret IntType, err error) {
	switch o.T__ {
	case IntType_BIG:
		if o.Big__ == nil {
			err = errors.New("unexpected nil value for Big__")
			return ret, err
		}
	case IntType_FIXED:
		if o.Fixed__ == nil {
			err = errors.New("unexpected nil value for Fixed__")
			return ret, err
		}
	}
	return o.T__, nil
}

func (o FlipParametersInt) Big() (res []byte) {
	if o.T__ != IntType_BIG {
		panic("wrong case accessed")
	}
	if o.Big__ == nil {
		return
	}
	return *o.Big__
}

func (o FlipParametersInt) Fixed() (res int64) {
	if o.T__ != IntType_FIXED {
		panic("wrong case accessed")
	}
	if o.Fixed__ == nil {
		return
	}
	return *o.Fixed__
}

func NewFlipParametersIntWithBig(v []byte) FlipParametersInt {
	return FlipParametersInt{
		T__:   IntType_BIG,
		Big__: &v,
	}
}

func NewFlipParametersIntWithFixed(v int64) FlipParametersInt {
	return FlipParametersInt{
		T__:     IntType_FIXED,
		Fixed__: &v,
	}
}

func NewFlipParametersIntWithBool() FlipParametersInt {
	return FlipParametersInt{
		T__: IntType_BOOL,
	}
}

func (o FlipParametersInt) DeepCopy() FlipParametersInt {
	return FlipParametersInt{
		T__: o.T__.DeepCopy(),
		Big__: (func(x *[]byte) *[]byte {
			if x == nil {
				return nil
			}
			tmp := (func(x []byte) []byte {
				if x == nil {
					return nil
				}
				return append([]byte{}, x...)
			})((*x))
			return &tmp
		})(o.Big__),
		Fixed__: (func(x *int64) *int64 {
			if x == nil {
				return nil
			}
			tmp := (*x)
			return &tmp
		})(o.Fixed__),
	}
}

type FlipParameters struct {
	T__       FlipType             `codec:"t" json:"t"`
	Ints__    *[]FlipParametersInt `codec:"ints,omitempty" json:"ints,omitempty"`
	Shuffle__ *int64               `codec:"shuffle,omitempty" json:"shuffle,omitempty"`
}

func (o *FlipParameters) T() (ret FlipType, err error) {
	switch o.T__ {
	case FlipType_INTS:
		if o.Ints__ == nil {
			err = errors.New("unexpected nil value for Ints__")
			return ret, err
		}
	case FlipType_SHUFFLE:
		if o.Shuffle__ == nil {
			err = errors.New("unexpected nil value for Shuffle__")
			return ret, err
		}
	}
	return o.T__, nil
}

func (o FlipParameters) Ints() (res []FlipParametersInt) {
	if o.T__ != FlipType_INTS {
		panic("wrong case accessed")
	}
	if o.Ints__ == nil {
		return
	}
	return *o.Ints__
}

func (o FlipParameters) Shuffle() (res int64) {
	if o.T__ != FlipType_SHUFFLE {
		panic("wrong case accessed")
	}
	if o.Shuffle__ == nil {
		return
	}
	return *o.Shuffle__
}

func NewFlipParametersWithInts(v []FlipParametersInt) FlipParameters {
	return FlipParameters{
		T__:    FlipType_INTS,
		Ints__: &v,
	}
}

func NewFlipParametersWithShuffle(v int64) FlipParameters {
	return FlipParameters{
		T__:       FlipType_SHUFFLE,
		Shuffle__: &v,
	}
}

func (o FlipParameters) DeepCopy() FlipParameters {
	return FlipParameters{
		T__: o.T__.DeepCopy(),
		Ints__: (func(x *[]FlipParametersInt) *[]FlipParametersInt {
			if x == nil {
				return nil
			}
			tmp := (func(x []FlipParametersInt) []FlipParametersInt {
				if x == nil {
					return nil
				}
				ret := make([]FlipParametersInt, len(x))
				for i, v := range x {
					vCopy := v.DeepCopy()
					ret[i] = vCopy
				}
				return ret
			})((*x))
			return &tmp
		})(o.Ints__),
		Shuffle__: (func(x *int64) *int64 {
			if x == nil {
				return nil
			}
			tmp := (*x)
			return &tmp
		})(o.Shuffle__),
	}
}

type MessageType int

const (
	MessageType_START               MessageType = 1
	MessageType_COMMITMENT          MessageType = 2
	MessageType_COMMITMENT_COMPLETE MessageType = 3
	MessageType_REVEAL              MessageType = 4
)

func (o MessageType) DeepCopy() MessageType { return o }

var MessageTypeMap = map[string]MessageType{
	"START":               1,
	"COMMITMENT":          2,
	"COMMITMENT_COMPLETE": 3,
	"REVEAL":              4,
}

var MessageTypeRevMap = map[MessageType]string{
	1: "START",
	2: "COMMITMENT",
	3: "COMMITMENT_COMPLETE",
	4: "REVEAL",
}

func (e MessageType) String() string {
	if v, ok := MessageTypeRevMap[e]; ok {
		return v
	}
	return ""
}

type Stage int

const (
	Stage_ROUND1 Stage = 1
	Stage_ROUND2 Stage = 2
)

func (o Stage) DeepCopy() Stage { return o }

var StageMap = map[string]Stage{
	"ROUND1": 1,
	"ROUND2": 2,
}

var StageRevMap = map[Stage]string{
	1: "ROUND1",
	2: "ROUND2",
}

func (e Stage) String() string {
	if v, ok := StageRevMap[e]; ok {
		return v
	}
	return ""
}

type Secret [32]byte

func (o Secret) DeepCopy() Secret {
	var ret Secret
	copy(ret[:], o[:])
	return ret
}

type Commitment [32]byte

func (o Commitment) DeepCopy() Commitment {
	var ret Commitment
	copy(ret[:], o[:])
	return ret
}

type CommitmentPayload struct {
	V Version    `codec:"v" json:"v"`
	U UserDevice `codec:"u" json:"u"`
	I GameID     `codec:"i" json:"i"`
	S Time       `codec:"s" json:"s"`
}

func (o CommitmentPayload) DeepCopy() CommitmentPayload {
	return CommitmentPayload{
		V: o.V.DeepCopy(),
		U: o.U.DeepCopy(),
		I: o.I.DeepCopy(),
		S: o.S.DeepCopy(),
	}
}

type GameMessageBody struct {
	T__                  MessageType         `codec:"t" json:"t"`
	Start__              *Start              `codec:"start,omitempty" json:"start,omitempty"`
	Commitment__         *Commitment         `codec:"commitment,omitempty" json:"commitment,omitempty"`
	CommitmentComplete__ *CommitmentComplete `codec:"commitmentComplete,omitempty" json:"commitmentComplete,omitempty"`
	Reveal__             *Secret             `codec:"reveal,omitempty" json:"reveal,omitempty"`
}

func (o *GameMessageBody) T() (ret MessageType, err error) {
	switch o.T__ {
	case MessageType_START:
		if o.Start__ == nil {
			err = errors.New("unexpected nil value for Start__")
			return ret, err
		}
	case MessageType_COMMITMENT:
		if o.Commitment__ == nil {
			err = errors.New("unexpected nil value for Commitment__")
			return ret, err
		}
	case MessageType_COMMITMENT_COMPLETE:
		if o.CommitmentComplete__ == nil {
			err = errors.New("unexpected nil value for CommitmentComplete__")
			return ret, err
		}
	case MessageType_REVEAL:
		if o.Reveal__ == nil {
			err = errors.New("unexpected nil value for Reveal__")
			return ret, err
		}
	}
	return o.T__, nil
}

func (o GameMessageBody) Start() (res Start) {
	if o.T__ != MessageType_START {
		panic("wrong case accessed")
	}
	if o.Start__ == nil {
		return
	}
	return *o.Start__
}

func (o GameMessageBody) Commitment() (res Commitment) {
	if o.T__ != MessageType_COMMITMENT {
		panic("wrong case accessed")
	}
	if o.Commitment__ == nil {
		return
	}
	return *o.Commitment__
}

func (o GameMessageBody) CommitmentComplete() (res CommitmentComplete) {
	if o.T__ != MessageType_COMMITMENT_COMPLETE {
		panic("wrong case accessed")
	}
	if o.CommitmentComplete__ == nil {
		return
	}
	return *o.CommitmentComplete__
}

func (o GameMessageBody) Reveal() (res Secret) {
	if o.T__ != MessageType_REVEAL {
		panic("wrong case accessed")
	}
	if o.Reveal__ == nil {
		return
	}
	return *o.Reveal__
}

func NewGameMessageBodyWithStart(v Start) GameMessageBody {
	return GameMessageBody{
		T__:     MessageType_START,
		Start__: &v,
	}
}

func NewGameMessageBodyWithCommitment(v Commitment) GameMessageBody {
	return GameMessageBody{
		T__:          MessageType_COMMITMENT,
		Commitment__: &v,
	}
}

func NewGameMessageBodyWithCommitmentComplete(v CommitmentComplete) GameMessageBody {
	return GameMessageBody{
		T__:                  MessageType_COMMITMENT_COMPLETE,
		CommitmentComplete__: &v,
	}
}

func NewGameMessageBodyWithReveal(v Secret) GameMessageBody {
	return GameMessageBody{
		T__:      MessageType_REVEAL,
		Reveal__: &v,
	}
}

func (o GameMessageBody) DeepCopy() GameMessageBody {
	return GameMessageBody{
		T__: o.T__.DeepCopy(),
		Start__: (func(x *Start) *Start {
			if x == nil {
				return nil
			}
			tmp := (*x).DeepCopy()
			return &tmp
		})(o.Start__),
		Commitment__: (func(x *Commitment) *Commitment {
			if x == nil {
				return nil
			}
			tmp := (*x).DeepCopy()
			return &tmp
		})(o.Commitment__),
		CommitmentComplete__: (func(x *CommitmentComplete) *CommitmentComplete {
			if x == nil {
				return nil
			}
			tmp := (*x).DeepCopy()
			return &tmp
		})(o.CommitmentComplete__),
		Reveal__: (func(x *Secret) *Secret {
			if x == nil {
				return nil
			}
			tmp := (*x).DeepCopy()
			return &tmp
		})(o.Reveal__),
	}
}

type Version int

const (
	Version_V1 Version = 1
)

func (o Version) DeepCopy() Version { return o }

var VersionMap = map[string]Version{
	"V1": 1,
}

var VersionRevMap = map[Version]string{
	1: "V1",
}

func (e Version) String() string {
	if v, ok := VersionRevMap[e]; ok {
		return v
	}
	return ""
}

type GameMessage struct {
	V__  Version        `codec:"v" json:"v"`
	V1__ *GameMessageV1 `codec:"v1,omitempty" json:"v1,omitempty"`
}

func (o *GameMessage) V() (ret Version, err error) {
	switch o.V__ {
	case Version_V1:
		if o.V1__ == nil {
			err = errors.New("unexpected nil value for V1__")
			return ret, err
		}
	}
	return o.V__, nil
}

func (o GameMessage) V1() (res GameMessageV1) {
	if o.V__ != Version_V1 {
		panic("wrong case accessed")
	}
	if o.V1__ == nil {
		return
	}
	return *o.V1__
}

func NewGameMessageWithV1(v GameMessageV1) GameMessage {
	return GameMessage{
		V__:  Version_V1,
		V1__: &v,
	}
}

func NewGameMessageDefault(v Version) GameMessage {
	return GameMessage{
		V__: v,
	}
}

func (o GameMessage) DeepCopy() GameMessage {
	return GameMessage{
		V__: o.V__.DeepCopy(),
		V1__: (func(x *GameMessageV1) *GameMessageV1 {
			if x == nil {
				return nil
			}
			tmp := (*x).DeepCopy()
			return &tmp
		})(o.V1__),
	}
}

type GameMessageV1 struct {
	GameID GameID          `codec:"gameID" json:"gameID"`
	Body   GameMessageBody `codec:"body" json:"body"`
}

func (o GameMessageV1) DeepCopy() GameMessageV1 {
	return GameMessageV1{
		GameID: o.GameID.DeepCopy(),
		Body:   o.Body.DeepCopy(),
	}
}

type FlipInterface interface {
}

func FlipProtocol(i FlipInterface) rpc.Protocol {
	return rpc.Protocol{
		Name:    "flip.flip",
		Methods: map[string]rpc.ServeHandlerDescription{},
	}
}

type FlipClient struct {
	Cli rpc.GenericClient
}
