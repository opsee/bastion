/* Messaging acts as a gateway between the Bastion and whatever is used
 * to communicate between Bastion components. It's responsibility is to provide
 * convenient interfaces to facilitate safe, structured message passing to
 * prevent lost/dead messages. All "wire-level" serialization/deserialization
 * should occur within the messaging package so that plugging different
 * messaging subsystems in (or ripping them out entirely) is easier.
 */
package messaging

import (
	"os"

	"github.com/opsee/bastion/logging"
)

// TODO: greg: Refactor Consumer/Producer into interfaces.

var (
	logger = logging.GetLogger("messaging")
)

func getNsqdURL() string {
	nsqdURL := os.Getenv("NSQD_HOST")
	return nsqdURL
}
