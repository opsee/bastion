package checks

import "github.com/opsee/bastion/Godeps/_workspace/src/golang.org/x/net/context"

type Runner interface {
    AddCheck(Check)
    RemoveCheck(Check)
    Run(context.Context)
}

type checkrunner struct {
    checks []Check
}

func (c *checkrunner) AddCheck(check Check) {

}

func (c *checkrunner) RemoveCheck(check Check) {

}

func (c *checkrunner) Run(context.Context) {
    resultsChan := make(chan CheckResults, len(c.checks))
    //    for _, c := range(c.checks) {
    //        ctx := context.WithValue(context.Background(), "results", resultsChan)
    //        c.Run(ctx)
    //    }
    var results []CheckResults
    waiting := len(c.checks)
    nchecks := waiting
    for waiting > 0 {
        res := <-resultsChan
        waiting--
        results[waiting-nchecks] = res
    }
}
