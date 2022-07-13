// Package stubs contains trivial stubs for parts of private.ImageDestination.
// It can be used from internal/wrapper, so it should not drag in any extra dependencies.
// Compare with imagedestination/impl, which might require non-trivial implementation work.
//
// There are two kinds of stubs:
// - Pure stubs, like ImplementsPutBlobPartial. Those can just be included in an imageDestination
//   implementation:
//
//   type yourDestination struct {
//       stubs.ImplementsPutBlobPartial
//       …
//   }
// - Stubs with a constructor, like NoPutBlobPartialInitialize. The Initialize marker
//   means that a constructor must be called:
//   type yourDestination struct {
//       stubs.NoPutBlobPartialInitialize
//       …
//   }
//
//   dest := &yourDestination{
//       …
//       NoPutBlobPartialInitialize: stubs.NoPutBlobPartial(ref),
//   }
//
package stubs
