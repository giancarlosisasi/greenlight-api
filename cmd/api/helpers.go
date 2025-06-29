package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

func (app *application) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
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
	err := json.NewDecoder(r.Body).Decode(dest)
	if err != nil {
		// If there is an error during decoding, start the triage
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

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

	return nil
}
