package wasmer

import (
	"errors"
	"fmt"
	"unsafe"
)

const OPCODE_COUNT = 410

// InstanceError represents any kind of errors related to a WebAssembly instance. It
// is returned by `Instance` functions only.
type InstanceError struct {
	// Error message.
	message string
}

// NewInstanceError constructs a new `InstanceError`.
func NewInstanceError(message string) *InstanceError {
	return &InstanceError{message}
}

// `InstanceError` is an actual error. The `Error` function returns
// the error message.
func (error *InstanceError) Error() string {
	return error.message
}

// ExportedFunctionError represents any kind of errors related to a
// WebAssembly exported function. It is returned by `Instance`
// functions only.
type ExportedFunctionError struct {
	functionName string
	message      string
}

// NewExportedFunctionError constructs a new `ExportedFunctionError`,
// where `functionName` is the name of the exported function, and
// `message` is the error message. If the error message contains `%s`,
// then this parameter will be replaced by `functionName`.
func NewExportedFunctionError(functionName string, message string) *ExportedFunctionError {
	return &ExportedFunctionError{functionName, message}
}

// ExportedFunctionError is an actual error. The `Error` function
// returns the error message.
func (error *ExportedFunctionError) Error() string {
	return fmt.Sprintf(error.message, error.functionName)
}

// Instance represents a WebAssembly instance.
type Instance struct {
	// The underlying WebAssembly instance.
	instance *cWasmerInstanceT

	// The imported functions. Use the `NewInstanceWithImports`
	// constructor to set it.
	imports *Imports

	// All functions exported by the WebAssembly instance, indexed
	// by their name as a string. An exported function is a
	// regular variadic Go closure. Arguments are untyped. Since
	// WebAssembly only supports: `i32`, `i64`, `f32` and `f64`
	// types, the accepted Go types are: `int8`, `uint8`, `int16`,
	// `uint16`, `int32`, `uint32`, `int64`, `int`, `uint`, `float32`
	// and `float64`. In addition to those types, the `Value` type
	// (from this project) is accepted. The conversion from a Go
	// value to a WebAssembly value is done automatically except for
	// the `Value` type (where type is coerced, that's the intent
	// here). The WebAssembly type is automatically inferred. Note
	// that the returned value is of kind `Value`, and not a
	// standard Go type.
	Exports map[string]func(...interface{}) (Value, error)

	// The exported memory of a WebAssembly instance.
	Memory *Memory
}

type ImportObject struct {
	imports         *Imports
	c_import_object *cWasmerImportObjectT
}

func NewImportObject(imports *Imports) (ImportObject, error) {
	var c_import_object *cWasmerImportObjectT

	wasmImportsCPointer, numberOfImports := generateWasmerImports(imports)

	var result = cWasmerNewImportObjectFromImports(
		&c_import_object,
		wasmImportsCPointer,
		cInt(numberOfImports),
	)

	if result != cWasmerOk {
		var lastError, err = GetLastError()
		var errorMessage = "Failed to create cached imports: %s"

		if err != nil {
			errorMessage = fmt.Sprintf(errorMessage, "(unknown details)")
		} else {
			errorMessage = fmt.Sprintf(errorMessage, lastError)
		}

		var emptyImportsCache = ImportObject{imports: nil, c_import_object: nil}
		return emptyImportsCache, errors.New(errorMessage)
	}

	importObject := ImportObject{imports: imports, c_import_object: c_import_object}
	return importObject, nil
}

// NewInstance constructs a new `Instance` with no imported functions.
func NewInstance(bytes []byte) (Instance, error) {
	return NewInstanceWithImports(bytes, NewImports())
}

// NewInstanceWithImports constructs a new `Instance` with imported functions.
func NewInstanceWithImports(bytes []byte, imports *Imports) (Instance, error) {
	wasmImportsCPointer, numberOfImports := generateWasmerImports(imports)

	var c_instance *cWasmerInstanceT

	var compileResult = cWasmerInstantiate(
		&c_instance,
		(*cUchar)(unsafe.Pointer(&bytes[0])),
		cUint(len(bytes)),
		wasmImportsCPointer,
		cInt(numberOfImports),
	)

	if compileResult != cWasmerOk {
		var lastError, err = GetLastError()
		var errorMessage = "Failed to instantiate the module:\n    %s"

		if err != nil {
			errorMessage = fmt.Sprintf(errorMessage, "(unknown details)")
		} else {
			errorMessage = fmt.Sprintf(errorMessage, lastError)
		}

		var emptyInstance = Instance{instance: nil, imports: nil, Exports: nil, Memory: nil}
		return emptyInstance, NewInstanceError(errorMessage)
	}

	instance, err := newInstanceWithImports(c_instance, imports)
	return instance, err
}

func NewMeteredInstanceWithImports(
	bytes []byte,
	imports *Imports,
	gasLimit uint64,
	opcode_costs *[OPCODE_COUNT]uint32,
) (Instance, error) {
	wasmImportsCPointer, numberOfImports := generateWasmerImports(imports)

	var c_instance *cWasmerInstanceT

	var compileResult = cWasmerInstantiateWithMetering(
		&c_instance,
		(*cUchar)(unsafe.Pointer(&bytes[0])),
		cUint(len(bytes)),
		wasmImportsCPointer,
		cInt(numberOfImports),
		gasLimit,
		opcode_costs,
	)

	if compileResult != cWasmerOk {
		var lastError, err = GetLastError()
		var errorMessage = "Failed to instantiate the module:\n    %s"

		if err != nil {
			errorMessage = fmt.Sprintf(errorMessage, "(unknown details)")
		} else {
			errorMessage = fmt.Sprintf(errorMessage, lastError)
		}

		var emptyInstance = Instance{instance: nil, imports: nil, Exports: nil, Memory: nil}
		return emptyInstance, NewInstanceError(errorMessage)
	}

	instance, err := newInstanceWithImports(c_instance, imports)
	return instance, err
}

func NewMeteredInstanceWithImportObject(
	bytes []byte,
	importObject *ImportObject,
	gasLimit uint64,
	opcode_costs *[OPCODE_COUNT]uint32,
) (Instance, error) {
	var c_instance *cWasmerInstanceT

	var compileResult = cWasmerInstantiateWithMeteringAndImportObject(
		&c_instance,
		(*cUchar)(unsafe.Pointer(&bytes[0])),
		cUint(len(bytes)),
		importObject.c_import_object,
		gasLimit,
		opcode_costs,
	)

	if compileResult != cWasmerOk {
		var lastError, err = GetLastError()
		var errorMessage = "Failed to instantiate the module:\n    %s"

		if err != nil {
			errorMessage = fmt.Sprintf(errorMessage, "(unknown details)")
		} else {
			errorMessage = fmt.Sprintf(errorMessage, lastError)
		}

		var emptyInstance = Instance{instance: nil, imports: nil, Exports: nil, Memory: nil}
		return emptyInstance, NewInstanceError(errorMessage)
	}

	instance, err := newInstanceWithImports(c_instance, importObject.imports)
	return instance, err
}

func newInstanceWithImports(
	c_instance *cWasmerInstanceT,
	imports *Imports,
) (Instance, error) {

	var emptyInstance = Instance{instance: nil, imports: nil, Exports: nil, Memory: nil}

	var wasmExports *cWasmerExportsT
	var hasMemory = false

	cWasmerInstanceExports(c_instance, &wasmExports)
	defer cWasmerExportsDestroy(wasmExports)

	exports, err := retrieveExportedFunctions(c_instance, wasmExports)
	if err != nil {
		return emptyInstance, err
	}

	memory, hasMemory, err := retrieveExportedMemory(wasmExports)
	if err != nil {
		return emptyInstance, err
	}

	if hasMemory == false {
		return Instance{instance: c_instance, imports: imports, Exports: exports, Memory: nil}, nil
	}

	return Instance{instance: c_instance, imports: imports, Exports: exports, Memory: &memory}, nil
}

// HasMemory checks whether the instance has at least one exported memory.
func (instance *Instance) HasMemory() bool {
	return nil != instance.Memory
}

// SetContextData assigns a data that can be used by all imported
// functions. Indeed, each imported function receives as its first
// argument an instance context (see `InstanceContext`). An instance
// context can hold a pointer to any kind of data. It is important to
// understand that this data is shared by all imported function, it's
// global to the instance.
func (instance *Instance) SetContextData(data unsafe.Pointer) {
	cWasmerInstanceContextDataSet(instance.instance, data)
}

// Close closes/frees an `Instance`.
func (instance *Instance) Close() {
	if instance.imports != nil {
		instance.imports.Close()
	}

	if instance.instance != nil {
		cWasmerInstanceDestroy(instance.instance)
	}
}

func (instance *Instance) Clean() {
	if instance.instance != nil {
		cWasmerInstanceDestroy(instance.instance)
	}
}

func (instance *Instance) GetPointsUsed() uint64 {
	return cWasmerInstanceGetPointsUsed(instance.instance)
}

func (instance *Instance) SetPointsUsed(points uint64) {
	cWasmerInstanceSetPointsUsed(instance.instance, points)
}
