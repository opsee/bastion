#Bastion Design#

##Requirements##

The bastion must be deployable as a single AMI.
The bastion must be able to reliably connect with Opsee's backend, or a proxy bastion, and operate independently in the event of a netsplit.
The bastion must be able to reliably run health checks against the backend services in a customer's infrastructure.
The bastion must be able to reliably route the results of those health checks to Opsee's backend.
Checks are defined to be run against a group. The bastion must be able to resolve different kinds of groups into a set of instances.  Initially the bastion will only be required to resolve security groups and a group of 1 individual instance.
When new instances are launched or old instances are retired from a group the bastion must send discovery events.
The bastion must be able to run a check in a test mode, where the check gets run against a small subset of instances and a single event is sent back as a reply.
A running bastion must be able to accept new types of checks without requiring a complete rebuild of its AMI.
The bastion will eventually need to be able to run commands and scripts to do things like restart instances and report the success or failure of that operation back to the user.

##Design##

In order to ensure the reliability of the various component pieces of the bastion, and to better ease development of future planned features, the bastion's architecture will mainly revolve around cooperating OS processes.  Therefore if a user defined check type hard crashes, it minimizes the extent of the failure.  The decoupling of responsibilities into cooperating processes is made tractable by adhearing as closely as possible to uniformity of interface in the data being exchanged.  Therefore we should be able to rapidly develop the following components, mostly gluing them together with off the shelf technology (either nsq or 0mq).

###Connector###

The connector process is responsible for maintaining communications with either the opsee backend, or the next bastion up in the tree if the bastions are in a tree structure.

###Majordomo###

The majordomo process is responsible for 