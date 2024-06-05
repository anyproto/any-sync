# Any-Sync
Any-Sync is an open-source protocol that enables local first communication and collaboration based on CRDTs. There are two important differentiators of Any-Sync:
- It is designed around creators’ controlled keys
- It is focused on bringing high-performance communication and collaboration at scale

Any-Sync fulfills the seven ideals of [local first software](https://www.inkandswitch.com/local-first/):
- **No spinners**: your work at your fingertips. Any-Sync keeps the primary copy of each space on the local device. Data synchronization with other devices happens quietly in the background - allowing you to operate with your data at your fingertips.
- **Your work is not trapped on one device.** Users can easily work on different devices. Each device keeps data in local storage, synchronization between devices happens in the background using CRDTs to resolve conflicts.
- **The network is optional.** Everything works offline. Data synchronization need not necessarily go via the Internet: Any-Sync allows users to sync data via local WiFi networks. Still, there is a role for the network - it works as additional backup, helps with peer discovery and especially solves the closed-laptop problem (you made changes on laptop, when your phone was offline, the changes can either sync when both devices are online or via backup node).
- **Seamless collaboration with your colleagues.** Achieving this goal is one of the biggest challenges in realizing local-first software, that’s why Any-Sync is built with CRDTs. So each device resolves conflicts independently.
- **The Long Now.** Because you have a local-first application, you can use it on your computer even if the software author disappears. This is also strengthened by open data standards and open code.
- **Security and privacy by default.** Any-Sync uses end-to-end encryption so that backup nodes store encrypted data that they cannot read. Conflict resolution happens on-device. The keys are controlled by users.
- **You retain ultimate ownership and control.** In the local first ideals this meant that you have local data, so you have ultimate ownership and control. To realize the idea of ultimate ownership we added creator controlled keys to Anytype. 

Additional two ideals that Any-Sync adds:
- **Creators’ controlled keys.** Creators control encryption keys; there is no central registry of users (we don’t even ask your email). We added an option to self-host your backup to support full autonomy of users from the network. 
- **Open Source.** Any-Sync protocol is open source, so all claims about how it works are independently verifiable.

We have released Anytype - the interface that is built on Any-Sync protocol. Users of Anytype can create spaces - graph-based databases with modular UI. Each space has unique access rights. 

## Introduction
We designed Any-Sync out of a strong conviction that the Internet today is akin to a nervous system of humanity - today it is cloud based, so all the neurons can communicate only via servers that are controlled by different elites. We envision a “no-one in between” local first alternative would be a much better foundation for communication on the internet. For this we’ve built Any-Sync to support fast and scalable synchronization of discussions, communities and apps.

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
