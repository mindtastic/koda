package logstore

import (
	"encoding/base64"
	"log"
	"testing"
)

// Valid base64 encoded record with key='valid_key' and value='valid_value'
const validRecordBase64 = "mFgwagAAAAAJAAAAC3ZhbGlkX2tleXZhbGlkX3ZhbHVl"

func TestDeserializeRecord(t *testing.T) {
	recordBytes, err := base64.RawStdEncoding.DecodeString(validRecordBase64)
	if err != nil {
		t.Fatalf("test setup invalid. Failed base64 decoding record string: %v", err)
	}

	record, err := Deserialize(recordBytes)
	if err != nil {
		t.Fatal("failed deserializing valid record: ", err)
	}

	t.Logf("Decoded record: %v", record)

	if record.kind != kindValue {
		t.Errorf("Unexpected record kind: %v (expected: %v)", record.kind, kindValue)
	}

	if record.key != "valid_key" {
		t.Errorf("Unexpected record key: %v (expected: %v)", record.key, "valid_key")
	}

	if string(record.value) != "valid_value" {
		t.Errorf("Unexpected record value: %v (expected: %v)", string(record.value), "valid_value")
	}
}

func TestSerializeRecord(t *testing.T) {
	record := &Record{
		kind:  kindValue,
		key:   "valid_key",
		value: []byte("valid_value"),
	}

	serialized := record.Serialize()
	b64 := base64.RawStdEncoding.EncodeToString(serialized)

	if b64 != validRecordBase64 {
		log.Fatalf("Serialized record does not match expectation: %v (expected: %v)", b64, validRecordBase64)
	}
}
