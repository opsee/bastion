# Bastion Design

## Requirements

The bastion must be able to:
  * be deployed as a single AMI.
  * reliably connect with Opsee's backend, or a proxy bastion, and operate independently in the event of a netsplit.
  * reliably run health checks against the backend services in a customer's infrastructure.
  * reliably route the results of those health checks to Opsee's backend.
  * resolve different kinds of groups into a set of instances.
  * resolve security groups and a group of 1 individual instance (v1).
  * send discovery events when group membership changes (instance addition/removal).
  * run a check in a test mode, where the check gets run against a small subset of instances and a single event is sent back as a reply.
  * accept new types of checks without requiring a complete rebuild of its AMI.

The bastion will eventually need to be able to run commands and scripts to do things like restart instances and report the success or failure of that operation back to the user.

## Design

In order to ensure the reliability of the various component pieces of the bastion, and to better ease development of future planned features, the bastion's architecture will mainly revolve around cooperating OS processes.  Therefore if a user defined check type hard crashes, it minimizes the extent of the failure.  The decoupling of responsibilities into cooperating processes is made tractable by adhearing as closely as possible to uniformity of interface in the data being exchanged.  Therefore we should be able to rapidly develop the following components, mostly gluing them together with off the shelf technology (either nsq or 0mq).

## Event Format

All events into and out of the bastion have a common format.  This eases development of the various components and eases the development of unplanned-for use cases.  Events can be batched in an envelope for more efficient delivery.  Events and envelopes are encoded on the wire as protocol buffers.

### Connector

The connector process is responsible for maintaining communications with either the opsee backend, or the next bastion up in the tree if the bastions are in a tree structure.  The connector negotiates the connection, sends up initial registration information that identifies this bastion or bastion network as belonging to a particular customer.  If, at any point in time, the connector becomes disconnected from the opsee backend, the connector will begin to reconnect with randomized exponential backoff.

### Event Bus

The event bus is the main means of exchanging data within the bastion.  The event bus is a pub-sub system where publishers will send messages to a topic, and consumers of that topic will have have those messages delivered to them.  One salient difference between the bastion event bus and most off the shelf pubsub systems is that the bastion bus implements the concept of a TTL.  The TTL means that a message will be persistent in a topic for the duration of the TTL.  Any subscribers to the topic will have any messages that are still within TTL delivered on subscribe.

### Majordoomo

The majordoomo process is responsible for scheduling checks that need to be run periodically. It will accept check definitions and begin scheduling those checks immediately. Fir v1, it will be limited to a very simple timer-based scheduler. It will utilize a caching layer between the bastion and AWS API to resolve group names into instance IP addresses.

### API Scanner

The API scanner is responsible for periodically scanning through the Amazon APIs, keeping information in memory about the instances reachable to this bastion, and publishing discovery events to the bastion's event bus. It is also responsible for populating the caching layer between the Bastion and AWS Describe API endpoints.

