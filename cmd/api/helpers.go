package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/giancarlosisasi/greenlight-api/internal/validator"
	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

func (app *application) readIDParam(r *http.Request) (string, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id := params.ByName("id")
	if id == "" {
		return "", errors.New("invalid id parameter")
	}

	return id, nil
}

func (app *application) writeJson(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	// remember that in real hight traffic apps, its better to use json.Marshal() because it has better performance than MarshalIndent
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	maps.Copy(w.Header(), headers)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dest any) error {
	// Use http.MaxBytesReader() to limit the size of the request body to 1,048,576 bytes (1mb)
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	// initialize the json.Decoder, and call the DisallowUnknownFields() method on it
	// before decoding. THis means that if the JSOn from the client now includes any
	// field that cannot mapped to the target destination, the decoder will return
	// an error instead of just ignoring the field.
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	// decode the request body to the destination
	err := dec.Decode(dest)

	if err != nil {
		// If there is an error during decoding, start the triage
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at the character %d)", syntaxError.Offset)
		// in some circumstances Decode() may also return an io.ErrUnexpectedEOF error
		// for syntax errors in the JSON. So we check for this using errors.Is() and
		// return a generic error message.
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		// Likewise, catch any json.UnmarshalTypeError errors. These occur when the
		// JSON value is the wrong type ofr the target destination. If the error relates
		// to a specific field, then we include in out error message to make it
		// easier for client to debug
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		// an io.EOF will be returned by Decode() if the request body is empty.
		// We check for this with errors.Is() and return a plain-english error message
		// instead
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		// if the json contains a field which cannot be mapped to the target destination
		// then Decode() will now return an error message in the format "json: unknown field "<name>""
		// We check for this, extract the field name from the error, and interpolate it into our custom error message.
		// Note that there's an open issue at https://github.com/golang/go/issues/29035 regarding thurning this
		// into a distinct error type in the future
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contains unknown keys %s", fieldName)

		// Use the errors.as() function to check whether the error has the type
		// *http.MaxBytesErrors. If it does, then it means the request body exceed our size limit of 1mb
		// and we return a clear error message
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)

		// A json.InvalidUnmarshalError error will be returned if we pass something
		// that is not a non-nil pointer as the target destination to Decode(). If this
		// happens we panic, rather that returning an error to our handler. At the end of
		// this chapter we'll briefly discuss why panicking is an appropriate thing to do
		// in this specific situation
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		// For any other error, return it as is
		default:
			return err
		}
	}

	// Call Decode() again, using a pointer to an empty anonymous struct as the destination.
	// if the request body only contained a single JSOn value this will return an io.EOF error.
	// So if we get anything else, we know that there is
	// additional data in the request body and we return our own custom error message.
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	return s
}

func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)

	if csv == "" {
		return defaultValue
	}

	return strings.Split(csv, ",")
}

func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	value := qs.Get(key)

	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}

	return intValue
}

func (app *application) background(fn func()) {
	app.wg.Add(1)
	// Launch a background goroutine
	go func() {
		// use defer to decrement the WaitGroup counter before the goroutines returns.
		defer app.wg.Done()

		// Recover any panic
		defer func() {
			pv := recover()
			if pv != nil {
				app.logger.Error(fmt.Sprintf("%v", pv))
			}
		}()

		// execute the arbitrary function that we passed as the parameter
		fn()
	}()
}
