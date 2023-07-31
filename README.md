# Any-Sync
Any-Sync is an open-source protocol designed to create high-performance, local-first, peer-to-peer, end-to-end encrypted applications that facilitate seamless collaboration among multiple users and devices. 

By utilizing this protocol, users can rest assured that they retain complete control over their data and digital experience. They are empowered to freely transition between various service providers, or even opt to self-host the applications.

This ensures utmost flexibility and autonomy for users in managing their personal information and digital interactions.

## Introduction
Most existing information management tools are implemented on centralized client-server architecture or designed for an offline-first single-user usage. Either way there are trade-offs for users: they can face restricted freedoms and privacy violations or compromise on the functionality of tools to avoid this.

We believe this goes against fundamental digital freedoms and that a new generation of software is needed that will respect these freedoms, while providing best in-class user experience. 

Our goal with `any-sync` is to develop a protocol that will enable the deployment of this software. 

Features: 
- Conflict-free data replication across multiple devices and agents
- Built-in end-to-end encryption
- Cryptographically verifiable history of changes
- Adoption to frequent operations (high performance) 
- Reliable and scalable infrastructure
- Simultaneous support of p2p and remote communication

## Protocol explanation
Plese read the [overview](https://tech.anytype.io/any-sync/overview) of protocol entities and design.

## Implementation

You can find the various parts of the protocol implemented in Go in the following repositories:
- [`any-sync-node`](https://github.com/anyproto/any-sync-node) — implementation of a sync node responsible for storing spaces and objects.
- [`any-sync-filenode`](https://github.com/anyproto/any-sync-filenode) — implementation of a file node responsible for storing files.
- [`any-sync-coordinator`](https://github.com/anyproto/any-sync-coordinator) — implementation of a coordinator node responsible for network configuration management.

## Contribution
Thank you for your desire to develop Anytype together!

❤️ This project and everyone involved in it is governed by the [Code of Conduct](https://github.com/anyproto/.github/blob/main/docs/CODE_OF_CONDUCT.md).

🧑‍💻 Check out our [contributing guide](https://github.com/anyproto/.github/blob/main/docs/CONTRIBUTING.md) to learn about asking questions, creating issues, or submitting pull requests.

🫢 For security findings, please email [security@anytype.io](mailto:security@anytype.io) and refer to our [security guide](https://github.com/anyproto/.github/blob/main/docs/SECURITY.md) for more information.

🤝 Follow us on [Github](https://github.com/anyproto) and join the [Contributors Community](https://github.com/orgs/anyproto/discussions).

---
Made by Any — a Swiss association 🇨🇭

Licensed under [MIT License](./LICENSE).