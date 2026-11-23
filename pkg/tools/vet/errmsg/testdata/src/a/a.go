package a

import "github.com/xhanio/errors"

const none = ""

func f(err error, msg string) []error {
	return []error{
		errors.Newf(""),                    // want `Newf\(""\) has no message`
		errors.Newf(none),                  // want `Newf\(""\) has no message`
		errors.NotFound.Newf(""),           // want `Newf\(""\) has no message`
		errors.Wrapf(err, ""),              // want `Wrapf\(err, ""\) adds a layer no reader sees`
		errors.NotFound.Wrapf(err, ""),     // want `Wrapf\(err, ""\) adds a layer no reader sees`
		errors.New(errors.WithMessage("")), // want `WithMessage\(""\) has no message`
		errors.New(),                       // want `errors.New\(\) has no message`

		errors.Newf("user %s not found", msg),
		errors.Newf(msg), // not a constant: its value is not known here
		errors.Wrapf(err, "load %s", msg),
		errors.Wrap(err),
		errors.New(errors.WithMessage("rate limited")),
		errors.NotFound.New(), // the category itself: a status alone
		errors.NotFound.Wrap(err),
		//framingo:ignore errmsg a placeholder the caller fills in
		errors.Newf(""),
	}
}
