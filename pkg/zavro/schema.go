package zavro

import (
	"crypto/md5"
	"fmt"
	"reflect"

	"github.com/go-avro/avro"
	"github.com/mccanne/zq/zng"
)

func GenSchema(typ zng.Type, namespace string) avro.Schema {
	switch typ := typ.(type) {
	case *zng.TypeRecord:
		return genRecordSchema(typ, namespace)
	case *zng.TypeVector:
		return genVectorSchema(typ, namespace)
	case *zng.TypeSet:
		return genSetSchema(typ, namespace)
	default:
		return genScalarSchema(typ)
	}
}

func genVectorSchema(typ *zng.TypeVector, namespace string) avro.Schema {
	inner := zng.InnerType(typ)
	return &avro.ArraySchema{
		Items: GenSchema(inner, namespace),
	}
}

func genSetSchema(typ *zng.TypeSet, namespace string) avro.Schema {
	// XXX this looks the same as vector for now but we will want to add
	// more meta-info to disnguish the two cases
	inner := zng.InnerType(typ)
	return &avro.ArraySchema{
		Items: GenSchema(inner, namespace),
	}
}

func genRecordSchema(typ *zng.TypeRecord, namespace string) avro.Schema {
	var fields []*avro.SchemaField
	for _, col := range typ.Columns {
		var union [2]avro.Schema
		union[0] = &avro.NullSchema{}
		union[1] = GenSchema(col.Type, namespace)
		fld := &avro.SchemaField{
			Name: col.Name,
			Type: &avro.UnionSchema{union[:]},
		}
		fields = append(fields, fld)
	}
	// We hash the zng type to an md5 fingerprint here, otherwise
	// we would get a ton of versions on the same name for different
	// instances/restarts of a zng stream.
	sum := md5.Sum([]byte(typ.String()))
	return &avro.RecordSchema{
		Name:       fmt.Sprintf("zng_%x", sum),
		Namespace:  namespace,
		Doc:        "Created by zinger from zng type " + typ.String(),
		Aliases:    nil,
		Properties: nil,
		Fields:     fields,
	}
}

func genScalarSchema(typ zng.Type) avro.Schema {
	switch typ.(type) {
	case *zng.TypeOfAddr:
		// IP addresses are turned into strings...
		return &avro.StringSchema{}

	case *zng.TypeOfBool:
		return &avro.BooleanSchema{}

	case *zng.TypeOfCount:
		return &avro.LongSchema{}

	case *zng.TypeOfDouble:
		return &avro.DoubleSchema{}

	case *zng.TypeOfEnum:
		// for now, we change zng enums to avro strings.
		// we would like to change enum to a conventional enum
		// but zeek doesn't provide the enum def so we just
		// cast zeek enum values to string values
		return &avro.StringSchema{}

	case *zng.TypeOfInt:
		// zng int is an avro long
		return &avro.LongSchema{}

	case *zng.TypeOfInterval:
		return &MicroTimeSchema{}

	case *zng.TypeOfPort:
		// XXX map a port to an int
		return &avro.IntSchema{}

	case *zng.TypeOfString:
		return &avro.StringSchema{}

	case *zng.TypeOfSubnet:
		return &avro.StringSchema{}

	case *zng.TypeOfTime:
		return &MicroTimeSchema{}

	default:
		panic("genScalarSchema: unknown type")
	}
}

// MicroTimeSchema implements avro.Schema and represents Avro long type.
type MicroTimeSchema struct{}

// Returns a JSON representation of LongSchema.
func (*MicroTimeSchema) String() string {
	return `{"type": "long", "name": "timestamp-micros", "logicalType": "timestamp-micros" }`
}

// Type returns a type constant for this MicroTimeSchema.
func (*MicroTimeSchema) Type() int {
	return avro.Long
}

// GetName returns a type name for this MicroTimeSchema.
func (*MicroTimeSchema) GetName() string {
	return "long"
}

// Prop doesn't return anything valuable for LongSchema.
func (*MicroTimeSchema) Prop(key string) (interface{}, bool) {
	return nil, false
}

// Validate checks whether the given value is writeable to this schema.
func (*MicroTimeSchema) Validate(v reflect.Value) bool {
	//return reflect.TypeOf(dereference(v).Interface()).Kind() == reflect.Int64
	//XXX
	return true
}

// MarshalJSON serializes the given schema as JSON. Never returns an error.
func (*MicroTimeSchema) MarshalJSON() ([]byte, error) {
	return []byte(`{ "type" : "long", "logicalType" : "timestamp-micros" }`), nil
}
