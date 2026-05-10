package service

import (
	"context"
	"sync"
)

const workflowRuntimeWorkerPoolSize = 2

type workflowWorkerPool struct {
	size int
}

func newWorkflowWorkerPool(size int) *workflowWorkerPool {
	if size <= 0 {
		size = 1
	}
	return &workflowWorkerPool{size: size}
}

func (p *workflowWorkerPool) Close() {}

func (p *workflowWorkerPool) ScheduleOrder(tokens []*WorkflowToken) []string {
	order := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == nil {
			continue
		}
		order = append(order, token.ID)
	}
	return order
}

func (p *workflowWorkerPool) Run(ctx context.Context, tokens []*WorkflowToken, run func(context.Context, *WorkflowToken) error) error {
	if p == nil {
		p = newWorkflowWorkerPool(1)
	}
	if len(tokens) == 0 {
		return nil
	}

	workerCount := p.size
	if workerCount > len(tokens) {
		workerCount = len(tokens)
	}

	results := make([]error, len(tokens))
	indices := make(chan int, len(tokens))
	for i := range tokens {
		indices <- i
	}
	close(indices)

	var wg sync.WaitGroup
	var once sync.Once
	var runErr error
	stop := make(chan struct{})
	worker := func() {
		defer wg.Done()
		for index := range indices {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			default:
			}
			err := run(ctx, tokens[index])
			results[index] = err
			if err != nil {
				once.Do(func() {
					runErr = err
					close(stop)
				})
				return
			}
		}
	}

	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go worker()
	}
	wg.Wait()
	if runErr != nil {
		return runErr
	}
	for _, err := range results {
		if err != nil {
			return err
		}
	}
	return nil
}
