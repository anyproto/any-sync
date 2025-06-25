# any-sync

`any-sync` is an open-source protocol designed for the post-cloud era, enabling high-speed, peer-to-peer synchronization of encrypted communication channels (spaces). It provides a communication layer for building private, decentralized applications offering unparalleled control, privacy, and performance.

## Core Principles

Each `any-sync` space (communication channel) is:

* **End-to-end encrypted**: Ensuring complete privacy of all messages and data.
* **User-owned**: Users maintain full control over their data and connections.
* **Permissionless**: Free from centralized lock-in, enabling true decentralization.

Thanks to its local-first design, `any-sync` achieves better-than-cloud performance when peers are physically close while maintaining global accessibility. At its core, data in `any-sync` is stored as encrypted Directed Acyclic Graphs (DAGs), representing various formats such as chats, pages, or databases. Its end-to-end encrypted structure ensures that no external entity can view a channel's content.

## Use Cases

`any-sync` enables developers to build private, decentralized alternatives to apps like Telegram, Discord, Notion, or even health-focused tools like Strava and Oura—without centralized infrastructure.

Paired with [any-store](https://sync.any.org(https://github.com/anyproto/any-store), it provides a local-first foundation for intelligent apps, including those using LLMs. Developers work with a local object store, while sync, communication, and scaling are handled seamlessly by the protocol—offering privacy, offline support, and strong performance with minimal complexity.

## Key Features

* **Encrypted, user-owned channels**: Maintain control over communication and data.
* **Permissionless operation**: No strict reliance on centralized services.
* **Local-first sync**: Functions offline and over peer-to-peer connections.
* **Seamless provider switching**: Retain access to channels even when changing sync providers.
* **Speed and security**: Combines fast performance with robust security.
* **Scalable infrastructure**: Efficiently supports large-scale collaboration, including large groups.

## Motivation

Traditional cloud infrastructures give corporations control over servers, causing entire communities to lose their connections and data if servers fail, get hacked, or block users. `any-sync` aims to bring the same degree of freedom to communication that blockchains brought to finance.

While blockchains excel at global consensus tasks like identity management, access control, or payments, they're not suited for real-time, end-to-end encrypted communication. Storing every encrypted message or document change on every node would be impractical. `any-sync` fills this gap by adding a fast, flexible communication layer that, paired with a decentralized, strictly ordered list, enables permissionless and zero-knowledge communication at scale.

## Operational Benefits

With `any-sync`:

* Users can choose and switch providers anytime without losing access or data.
* Providers cannot read user information, block users, or alter accounts—they only deliver sync and storage.

`any-sync` channels can scale without practical limits thanks to a Conflict-free Replicated Data Type (CRDT)-based, gas-less mechanism cryptographically signing every change in its DAGs. Each device independently applies and cryptographically verifies CRDT updates, ensuring consistent final states without traditional consensus protocols.

By supporting multiple data formats (chats, pages, databases) and storing files externally (e.g., via IPFS), `any-sync` provides a flexible foundation for secure, decentralized communication.


## Protocol overview
Plese read the [overview](https://sync.any.org) of protocol entities and design.

## Implementation

You can find the various parts of the protocol implemented in Go in the following repositories:
- [`any-sync-node`](https://github.com/anyproto/any-sync-node) — implementation of a sync node responsible for storing spaces and objects.
- [`any-sync-filenode`](https://github.com/anyproto/any-sync-filenode) — implementation of a file node responsible for storing files.
- [`any-sync-consensusnode`](https://github.com/anyproto/any-sync-consensusnode) — implementation of a consensus node responsible for ACL changes monitoring and validation.
- [`any-sync-coordinator`](https://github.com/anyproto/any-sync-coordinator) — implementation of a coordinator node responsible for network configuration management.

## Contribution
Thank you for your desire to develop Anytype together!

❤️ This project and everyone involved in it is governed by the [Code of Conduct](https://github.com/anyproto/.github/blob/main/docs/CODE_OF_CONDUCT.md).

🧑‍💻 Check out our [contributing guide](https://github.com/anyproto/.github/blob/main/docs/CONTRIBUTING.md) to learn about asking questions, creating issues, or submitting pull requests.

🫢 For security findings, please email [security@anytype.io](mailto:security@anytype.io) and refer to our [security guide](https://github.com/anyproto/.github/blob/main/docs/SECURITY.md) for more information.

🤝 Follow us on [Github](https://github.com/anyproto) and join the [Contributors Community](https://github.com/orgs/anyproto/discussions).

---
Made by Any — a Swiss association 🇨🇭

Licensed under [MIT License](./LICENSE.md).
