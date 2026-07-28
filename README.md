# chargeflow-registry

[![Test](https://github.com/ChargePi/chargeflow-registry/actions/workflows/test.yaml/badge.svg)](https://github.com/ChargePi/chargeflow-registry/actions/workflows/test.yaml)
[![Lint](https://github.com/ChargePi/chargeflow-registry/actions/workflows/lint.yaml/badge.svg)](https://github.com/ChargePi/chargeflow-registry/actions/workflows/lint.yaml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ChargePi/chargeflow-registry)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE.md)

A dedicated OCPP schema registry that allows compatibility checks for both CPMS and EV chargers.

OCPP is a standard in name only — every vendor bends it a little. Chargeflow Registry gives you one source of truth for
what "valid" means per version, action, and vendor, so mismatches get caught before they cause a failed charging
session, not after. That matters most at two points that otherwise cost teams real time and money:

- **Integration** — onboarding a new charge point model or CPMS shouldn't mean weeks of manual payload comparison.
  Validate against the registry during integration testing and catch schema drift immediately, instead of debugging it
  in production.
- **Compliance** — OCPP 1.6, 2.0.1, and 2.1 conformance is a moving target, and audits or certification reviews need
  proof, not assurances. A central, versioned registry gives you an auditable record of exactly which schema a given
  vendor/model was validated against.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE.md) file for details.

## Contributing

We welcome contributions to this project! Please read our [contributing guidelines](CONTRIBUTING.md) for more
information on how to get started, and our [Code of Conduct](CODE_OF_CONDUCT.md) before participating.