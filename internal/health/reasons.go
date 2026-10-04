package health

// Explanation says what an operator condition reason means and, when a
// person can act on it, what to do next. The text is shown to users as is.
type Explanation struct {
	Meaning  string
	NextStep string
}

// Explain returns the explanation for an operator condition reason. A reason
// the portal does not know returns false; callers show it raw.
func Explain(reason string) (Explanation, bool) {
	e, ok := reasonExplanations[reason]
	return e, ok
}

// reasonExplanations holds one row per reason the operator writes into a
// condition. The test suite compares its keys with the operator's reason
// constants.
var reasonExplanations = map[string]Explanation{
	// Progress and success.
	"Progressing": {
		Meaning: "The operator is working on the latest change.",
	},
	"ReconciliationSucceeded": {
		Meaning: "Every object was applied. This says nothing about whether the workloads run; see Health.",
	},
	"ModuleResolved": {
		Meaning: "The module version was found in the registry.",
	},
	"Suspended": {
		Meaning: "Reconciliation is suspended; the operator applies nothing until it is resumed.",
	},
	"ManagedExternally": {
		Meaning: "The CLI applied this instance; the operator only records it.",
	},
	"DriftDetected": {
		Meaning:  "Some applied objects differ from what the operator applied last.",
		NextStep: "Inspect the drifted objects; the next reconcile reapplies them.",
	},

	// ModuleInstance and ModulePackage failures.
	"ResolutionFailed": {
		Meaning:  "The module could not be resolved from the registry.",
		NextStep: "Check the module path and version, and that the registry is reachable.",
	},
	"RenderFailed": {
		Meaning:  "The module did not render against the platform.",
		NextStep: "Read the message for the failing value or definition and fix the module or its values.",
	},
	"SkewRefused": {
		Meaning:  "The module needs a newer catalog build than the platform pins, and the platform refuses skew.",
		NextStep: "Raise the platform's catalog version, or use an older module version.",
	},
	"DuplicateIdentities": {
		Meaning:  "Two or more rendered objects share one Kubernetes identity, so nothing was applied.",
		NextStep: "Remove or rename one of the components the message names.",
	},
	"ApplyFailed": {
		Meaning:  "Applying the rendered objects failed.",
		NextStep: "Read the message; a Forbidden error usually means the applying ServiceAccount lacks permissions, for example for cluster-scoped objects.",
	},
	"PruneFailed": {
		Meaning:  "Objects no longer rendered could not be deleted.",
		NextStep: "Check that the applying ServiceAccount may delete the objects the message names.",
	},
	"ImpersonationFailed": {
		Meaning:  "The operator could not act as the ServiceAccount the spec names.",
		NextStep: "Check that the ServiceAccount exists and that the operator may impersonate it.",
	},
	"DeletionSAMissing": {
		Meaning:  "Deletion is blocked: the ServiceAccount needed to prune the objects is gone.",
		NextStep: "Restore the ServiceAccount, set spec.prune to false and delete again, or annotate the object with opm.dev/force-delete-orphan=true to leave the objects behind.",
	},
	"SelfManagementRefused": {
		Meaning:  "This instance deploys the operator itself, which the operator never manages.",
		NextStep: "Set spec.owner to cli.",
	},
	"PlatformNotReady": {
		Meaning:  "The Platform is not ready, so nothing can be rendered against it yet.",
		NextStep: "Look at the Platform's status.",
	},
	"DependentsRemain": {
		Meaning:  "Removing this provider, or a contract it provides, is refused while instances still demand it.",
		NextStep: "Remove the dependent instances the message names first.",
	},

	// ModulePackage.
	"SourceNotReady": {
		Meaning:  "The package's source is not ready or does not exist.",
		NextStep: "Check the source object the package names; the operator retries.",
	},
	"FetchFailed": {
		Meaning:  "The package's artifact could not be fetched.",
		NextStep: "Check the source's artifact URL and that the operator can reach it.",
	},
	"PathNotFound": {
		Meaning:  "The path the package names does not exist in the artifact.",
		NextStep: "Fix spec.path.",
	},
	"InstanceFileNotFound": {
		Meaning:  "The package path holds no instance file.",
		NextStep: "Check that the artifact contains the instance file at spec.path.",
	},
	"UnsupportedKind": {
		Meaning:  "The package holds a kind the operator does not deploy.",
		NextStep: "Package a supported kind.",
	},
	"DependenciesNotReady": {
		Meaning:  "A package this one depends on is not ready yet.",
		NextStep: "Look at the packages in spec.dependsOn.",
	},

	// Platform.
	"Generated": {
		Meaning: "The platform module was generated and built.",
	},
	"GenerateFailed": {
		Meaning:  "The platform module could not be written.",
		NextStep: "Read the message; this is usually an operator storage problem.",
	},
	"BuildFailed": {
		Meaning:  "The platform module did not build: a catalog did not resolve, or its contracts could not be read.",
		NextStep: "Check the catalog versions in the registry subscriptions and that the registry is reachable.",
	},
	"ContractCollisions": {
		Meaning:  "More than one enabled catalog defines the same contract.",
		NextStep: "Disable all but one of the registry entries the message names.",
	},
	"OverSubscribedContracts": {
		Meaning:  "A provider contract is provided by more than one enabled catalog or registration.",
		NextStep: "Disable a competing catalog or remove its registration.",
	},
	"ComparablePredicates": {
		Meaning:  "Two transformers would both render the same components.",
		NextStep: "Narrow or withdraw one of the transformers the message names.",
	},
	"UnfulfilledContracts": {
		Meaning: "Some provider contracts have no provider yet. This is normal on a new platform; a module that needs one waits until a provider registers.",
	},
	"ContractsFulfilled": {
		Meaning: "Every provider contract the platform defines has a provider.",
	},
	"NoContractsDefined": {
		Meaning: "The enabled catalogs define no contracts, so nothing was checked.",
	},

	// TransformerRegistration.
	"Accepted": {
		Meaning: "The registration passed every check. Whether it is active is shown separately.",
	},
	"CatalogUnresolved": {
		Meaning:  "The registered catalog and version resolve to nothing.",
		NextStep: "Check the catalog path and version, including the version's format, and the registry.",
	},
	"CatalogWrongKind": {
		Meaning:  "The registered module is not a catalog.",
		NextStep: "Register a catalog module.",
	},
	"ProvidesMismatch": {
		Meaning:  "The contracts the registration lists differ from the ones its catalog implements.",
		NextStep: "List exactly the contracts the catalog implements.",
	},
	"ProviderMismatch": {
		Meaning:  "The registration did not come from the provider instance it names.",
		NextStep: "Let the provider module render the registration instead of applying it by hand.",
	},
	"ProviderInventoryPending": {
		Meaning: "The provider instance has not recorded its objects yet; the verdict follows.",
	},
	"BuildIncompatible": {
		Meaning:  "The catalog needs a shared dependency version the platform did not resolve to.",
		NextStep: "Align the catalog with the platform's resolved versions.",
	},
	"ProviderReady": {
		Meaning: "The provider is ready, so the registration is active.",
	},
	"ProviderNotReady": {
		Meaning:  "The registration is accepted, and its provider instance is not ready yet.",
		NextStep: "Look at the provider instance's status.",
	},
	"ContractSubscribed": {
		Meaning:  "A catalog the platform subscribes to already provides one of these contracts.",
		NextStep: "Remove that subscription from the Platform, or drop the contract from this registration.",
	},
	"ContractClaimed": {
		Meaning:  "Another active registration already provides one of these contracts.",
		NextStep: "Remove the other provider first.",
	},
	"DuplicateClaim": {
		Meaning:  "Another registration already holds this catalog.",
		NextStep: "Remove the registration the message names, or this one.",
	},
}
