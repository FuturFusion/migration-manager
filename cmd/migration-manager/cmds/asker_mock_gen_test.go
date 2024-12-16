// Code generated by moq; DO NOT EDIT.
// github.com/matryer/moq

package cmds

import (
	"sync"
)

// Ensure, that AskerMock does implement Asker.
// If this is not the case, regenerate this file with moq.
var _ Asker = &AskerMock{}

// AskerMock is a mock implementation of Asker.
//
//	func TestSomethingThatUsesAsker(t *testing.T) {
//
//		// make and configure a mocked Asker
//		mockedAsker := &AskerMock{
//			AskBoolFunc: func(question string, defaultAnswer string) (bool, error) {
//				panic("mock out the AskBool method")
//			},
//			AskChoiceFunc: func(question string, choices []string, defaultAnswer string) (string, error) {
//				panic("mock out the AskChoice method")
//			},
//			AskIntFunc: func(question string, minValue int64, maxValue int64, defaultAnswer string, validate func(int64) error) (int64, error) {
//				panic("mock out the AskInt method")
//			},
//			AskPasswordFunc: func(question string) string {
//				panic("mock out the AskPassword method")
//			},
//			AskStringFunc: func(question string, defaultAnswer string, validate func(string) error) (string, error) {
//				panic("mock out the AskString method")
//			},
//		}
//
//		// use mockedAsker in code that requires Asker
//		// and then make assertions.
//
//	}
type AskerMock struct {
	// AskBoolFunc mocks the AskBool method.
	AskBoolFunc func(question string, defaultAnswer string) (bool, error)

	// AskChoiceFunc mocks the AskChoice method.
	AskChoiceFunc func(question string, choices []string, defaultAnswer string) (string, error)

	// AskIntFunc mocks the AskInt method.
	AskIntFunc func(question string, minValue int64, maxValue int64, defaultAnswer string, validate func(int64) error) (int64, error)

	// AskPasswordFunc mocks the AskPassword method.
	AskPasswordFunc func(question string) string

	// AskStringFunc mocks the AskString method.
	AskStringFunc func(question string, defaultAnswer string, validate func(string) error) (string, error)

	// calls tracks calls to the methods.
	calls struct {
		// AskBool holds details about calls to the AskBool method.
		AskBool []struct {
			// Question is the question argument value.
			Question string
			// DefaultAnswer is the defaultAnswer argument value.
			DefaultAnswer string
		}
		// AskChoice holds details about calls to the AskChoice method.
		AskChoice []struct {
			// Question is the question argument value.
			Question string
			// Choices is the choices argument value.
			Choices []string
			// DefaultAnswer is the defaultAnswer argument value.
			DefaultAnswer string
		}
		// AskInt holds details about calls to the AskInt method.
		AskInt []struct {
			// Question is the question argument value.
			Question string
			// MinValue is the minValue argument value.
			MinValue int64
			// MaxValue is the maxValue argument value.
			MaxValue int64
			// DefaultAnswer is the defaultAnswer argument value.
			DefaultAnswer string
			// Validate is the validate argument value.
			Validate func(int64) error
		}
		// AskPassword holds details about calls to the AskPassword method.
		AskPassword []struct {
			// Question is the question argument value.
			Question string
		}
		// AskString holds details about calls to the AskString method.
		AskString []struct {
			// Question is the question argument value.
			Question string
			// DefaultAnswer is the defaultAnswer argument value.
			DefaultAnswer string
			// Validate is the validate argument value.
			Validate func(string) error
		}
	}
	lockAskBool     sync.RWMutex
	lockAskChoice   sync.RWMutex
	lockAskInt      sync.RWMutex
	lockAskPassword sync.RWMutex
	lockAskString   sync.RWMutex
}

// AskBool calls AskBoolFunc.
func (mock *AskerMock) AskBool(question string, defaultAnswer string) (bool, error) {
	if mock.AskBoolFunc == nil {
		panic("AskerMock.AskBoolFunc: method is nil but Asker.AskBool was just called")
	}
	callInfo := struct {
		Question      string
		DefaultAnswer string
	}{
		Question:      question,
		DefaultAnswer: defaultAnswer,
	}
	mock.lockAskBool.Lock()
	mock.calls.AskBool = append(mock.calls.AskBool, callInfo)
	mock.lockAskBool.Unlock()
	return mock.AskBoolFunc(question, defaultAnswer)
}

// AskBoolCalls gets all the calls that were made to AskBool.
// Check the length with:
//
//	len(mockedAsker.AskBoolCalls())
func (mock *AskerMock) AskBoolCalls() []struct {
	Question      string
	DefaultAnswer string
} {
	var calls []struct {
		Question      string
		DefaultAnswer string
	}
	mock.lockAskBool.RLock()
	calls = mock.calls.AskBool
	mock.lockAskBool.RUnlock()
	return calls
}

// AskChoice calls AskChoiceFunc.
func (mock *AskerMock) AskChoice(question string, choices []string, defaultAnswer string) (string, error) {
	if mock.AskChoiceFunc == nil {
		panic("AskerMock.AskChoiceFunc: method is nil but Asker.AskChoice was just called")
	}
	callInfo := struct {
		Question      string
		Choices       []string
		DefaultAnswer string
	}{
		Question:      question,
		Choices:       choices,
		DefaultAnswer: defaultAnswer,
	}
	mock.lockAskChoice.Lock()
	mock.calls.AskChoice = append(mock.calls.AskChoice, callInfo)
	mock.lockAskChoice.Unlock()
	return mock.AskChoiceFunc(question, choices, defaultAnswer)
}

// AskChoiceCalls gets all the calls that were made to AskChoice.
// Check the length with:
//
//	len(mockedAsker.AskChoiceCalls())
func (mock *AskerMock) AskChoiceCalls() []struct {
	Question      string
	Choices       []string
	DefaultAnswer string
} {
	var calls []struct {
		Question      string
		Choices       []string
		DefaultAnswer string
	}
	mock.lockAskChoice.RLock()
	calls = mock.calls.AskChoice
	mock.lockAskChoice.RUnlock()
	return calls
}

// AskInt calls AskIntFunc.
func (mock *AskerMock) AskInt(question string, minValue int64, maxValue int64, defaultAnswer string, validate func(int64) error) (int64, error) {
	if mock.AskIntFunc == nil {
		panic("AskerMock.AskIntFunc: method is nil but Asker.AskInt was just called")
	}
	callInfo := struct {
		Question      string
		MinValue      int64
		MaxValue      int64
		DefaultAnswer string
		Validate      func(int64) error
	}{
		Question:      question,
		MinValue:      minValue,
		MaxValue:      maxValue,
		DefaultAnswer: defaultAnswer,
		Validate:      validate,
	}
	mock.lockAskInt.Lock()
	mock.calls.AskInt = append(mock.calls.AskInt, callInfo)
	mock.lockAskInt.Unlock()
	return mock.AskIntFunc(question, minValue, maxValue, defaultAnswer, validate)
}

// AskIntCalls gets all the calls that were made to AskInt.
// Check the length with:
//
//	len(mockedAsker.AskIntCalls())
func (mock *AskerMock) AskIntCalls() []struct {
	Question      string
	MinValue      int64
	MaxValue      int64
	DefaultAnswer string
	Validate      func(int64) error
} {
	var calls []struct {
		Question      string
		MinValue      int64
		MaxValue      int64
		DefaultAnswer string
		Validate      func(int64) error
	}
	mock.lockAskInt.RLock()
	calls = mock.calls.AskInt
	mock.lockAskInt.RUnlock()
	return calls
}

// AskPassword calls AskPasswordFunc.
func (mock *AskerMock) AskPassword(question string) string {
	if mock.AskPasswordFunc == nil {
		panic("AskerMock.AskPasswordFunc: method is nil but Asker.AskPassword was just called")
	}
	callInfo := struct {
		Question string
	}{
		Question: question,
	}
	mock.lockAskPassword.Lock()
	mock.calls.AskPassword = append(mock.calls.AskPassword, callInfo)
	mock.lockAskPassword.Unlock()
	return mock.AskPasswordFunc(question)
}

// AskPasswordCalls gets all the calls that were made to AskPassword.
// Check the length with:
//
//	len(mockedAsker.AskPasswordCalls())
func (mock *AskerMock) AskPasswordCalls() []struct {
	Question string
} {
	var calls []struct {
		Question string
	}
	mock.lockAskPassword.RLock()
	calls = mock.calls.AskPassword
	mock.lockAskPassword.RUnlock()
	return calls
}

// AskString calls AskStringFunc.
func (mock *AskerMock) AskString(question string, defaultAnswer string, validate func(string) error) (string, error) {
	if mock.AskStringFunc == nil {
		panic("AskerMock.AskStringFunc: method is nil but Asker.AskString was just called")
	}
	callInfo := struct {
		Question      string
		DefaultAnswer string
		Validate      func(string) error
	}{
		Question:      question,
		DefaultAnswer: defaultAnswer,
		Validate:      validate,
	}
	mock.lockAskString.Lock()
	mock.calls.AskString = append(mock.calls.AskString, callInfo)
	mock.lockAskString.Unlock()
	return mock.AskStringFunc(question, defaultAnswer, validate)
}

// AskStringCalls gets all the calls that were made to AskString.
// Check the length with:
//
//	len(mockedAsker.AskStringCalls())
func (mock *AskerMock) AskStringCalls() []struct {
	Question      string
	DefaultAnswer string
	Validate      func(string) error
} {
	var calls []struct {
		Question      string
		DefaultAnswer string
		Validate      func(string) error
	}
	mock.lockAskString.RLock()
	calls = mock.calls.AskString
	mock.lockAskString.RUnlock()
	return calls
}
