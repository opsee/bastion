package schema

import (
	"reflect"

	opsee_types "github.com/opsee/protobuf/opseeproto/types"
)

// register types
func init() {
	opsee_types.AnyTypeRegistry.RegisterAny("CloudWatchCheck", reflect.TypeOf(CloudWatchCheck{}))
	opsee_types.AnyTypeRegistry.RegisterAny("CloudWatchResponse", reflect.TypeOf(CloudWatchResponse{}))
	opsee_types.AnyTypeRegistry.RegisterAny("HttpCheck", reflect.TypeOf(HttpCheck{}))
	opsee_types.AnyTypeRegistry.RegisterAny("HttpResponse", reflect.TypeOf(HttpResponse{}))
}
