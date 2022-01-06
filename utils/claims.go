package utils

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	core "github.com/iden3/go-iden3-core"
	"github.com/iden3/go-schema-processor/processor"
	"github.com/iden3/go-schema-processor/verifiable"
	"github.com/pkg/errors"
	"math/big"
	"strconv"
)

var q *big.Int

//nolint //reason - needed
func init() {
	qString := "21888242871839275222246405745257275088548364400416034343698204186575808495617"
	var ok bool
	q, ok = new(big.Int).SetString(qString, 10)
	if !ok {
		panic(fmt.Sprintf("Bad base 10 string %s", qString))
	}
}

// FieldToByteArray convert fields to byte representation based on type
func FieldToByteArray(field interface{}) ([]byte, error) {

	switch v := field.(type) {
	case uint32:
		bs := make([]byte, 4)
		binary.LittleEndian.PutUint32(bs, v)
		return bs, nil
	case float64:
		s := fmt.Sprintf("%.0f", v)
		intValue, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("can not convert field %v to uint32", field)
		}

		bs := make([]byte, 4)
		binary.LittleEndian.PutUint32(bs, uint32(intValue))
		return bs, nil
	}

	return nil, fmt.Errorf("not supported field type %T", field)
}

// DataFillsSlot  checks if newData fills into slot capacity ()
func DataFillsSlot(slot, newData []byte) bool {
	slot = append(slot, newData...)
	a := new(big.Int).SetBytes(swapEndianness(slot))
	return a.Cmp(q) == -1
}

// CheckDataInField  checks if data is in Q field
func CheckDataInField(data []byte) bool {
	a := new(big.Int).SetBytes(swapEndianness(data))
	return a.Cmp(q) == -1
}

var errIndexHashNotEqual = errors.New("claim index hash in credential and in resulted claim are not equal")
var errValueHashNotEqual = errors.New("claim value hash in credential and in resulted claim are not equal")

// FillClaimSlots fullfils index and value fields to iden3 slots
func FillClaimSlots(content []byte, indexFields, valueFields []string) (processor.ParsedSlots, error) {
	var data map[string]interface{}

	err := json.Unmarshal(content, &data)
	if err != nil {
		return processor.ParsedSlots{}, err
	}

	slotAFilled := false
	result := processor.ParsedSlots{
		IndexA: make([]byte, 0, 32),
		IndexB: make([]byte, 0, 32),
		ValueA: make([]byte, 0, 32),
		ValueB: make([]byte, 0, 32),
	}

	for _, key := range indexFields {
		// key is a property of data map to process
		byteValue, err := FieldToByteArray(data[key])
		if err != nil {
			return processor.ParsedSlots{}, err
		}

		if !slotAFilled {
			if DataFillsSlot(result.IndexA, byteValue) {
				result.IndexA = append(result.IndexA, byteValue...)
				continue
			} else {
				slotAFilled = true
			}
		}

		if DataFillsSlot(result.IndexB, byteValue) {
			result.IndexB = append(result.IndexB, byteValue...)
		} else {
			return processor.ParsedSlots{}, processor.ErrSlotsOverflow
		}
	}

	slotAFilled = false
	for _, key := range valueFields {
		// key is a property of data map to process
		byteValue, err := FieldToByteArray(data[key])
		if err != nil {
			return processor.ParsedSlots{}, err
		}
		if !slotAFilled {
			if DataFillsSlot(result.ValueA, byteValue) {
				result.ValueA = append(result.ValueA, byteValue...)
				continue
			} else {
				slotAFilled = true
			}
		}

		if DataFillsSlot(result.ValueB, byteValue) {
			result.ValueB = append(result.ValueB, byteValue...)
		} else {
			return processor.ParsedSlots{}, processor.ErrSlotsOverflow
		}
	}

	return result, nil
}

func swapEndianness(buf []byte) []byte {
	newBuf := make([]byte, len(buf))
	for i, b := range buf {
		newBuf[len(buf)-1-i] = b
	}
	return newBuf
}

// IndexOf returns field index in array of fields
func IndexOf(field string, fields []string) int {
	for k, v := range fields {
		if field == v {
			return k
		}
	}
	return -1
}

// CreateSchemaHash calculates schema hash
func CreateSchemaHash(credentialType string) core.SchemaHash {
	var sHash core.SchemaHash
	h := Keccak256([]byte(credentialType))
	copy(sHash[:], h[len(h)-16:])
	return sHash
}

// VerifyClaimHash verifies that hashes of index and value of claim are equal to proof in credential
func VerifyClaimHash(credential *verifiable.Iden3Credential, claim *core.Claim) error {

	entry := claim.TreeEntry()
	hi, hv, err := entry.HiHv()

	if err != nil {
		return err
	}
	switch proof := credential.Proof.(type) {
	case []verifiable.BasicProof:
		for _, p := range proof {
			if p.HIndex != hi.Hex() {
				return errIndexHashNotEqual
			}
			if p.HValue != hv.Hex() {
				return errValueHashNotEqual
			}
		}
	case verifiable.BasicProof:
		if proof.HIndex != hi.Hex() {
			return errIndexHashNotEqual
		}
		if proof.HValue != hv.Hex() {
			return errValueHashNotEqual
		}
	}
	return nil

}