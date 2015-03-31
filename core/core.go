package core

type IService interface {
    Start()
    Stop()

    State() int

    SetName(name string)
    Name() string

    SetParent(parent IService)
    Parent() IService
}


