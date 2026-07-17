package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type getCredentialsFunc func(ctx context.Context) (*Credentials, error)

type Updater struct {
	expiryWindow  time.Duration
	refreshPeriod time.Duration

	// for fix below case:
	// * both auth.Signer and credential.Credential are not concurrent safe
	expiryWindowForRefreshLoop time.Duration

	getCredentials func(ctx context.Context) (*Credentials, error)

	cred        *Credentials
	lockForCred sync.RWMutex

	Logger    Logger
	nowFunc   func() time.Time
	logPrefix string

	doneCh  chan struct{}
	stopped bool
}

type UpdaterOptions struct {
	ExpiryWindow  time.Duration
	RefreshPeriod time.Duration
	Logger        Logger
	LogPrefix     string
}

func NewUpdater(getter getCredentialsFunc, opts UpdaterOptions) *Updater {
	u := &Updater{
		expiryWindow:               opts.ExpiryWindow,
		refreshPeriod:              opts.RefreshPeriod,
		expiryWindowForRefreshLoop: opts.RefreshPeriod + opts.RefreshPeriod/2,
		getCredentials:             getter,
		cred:                       nil,
		lockForCred:                sync.RWMutex{},
		Logger:                     opts.Logger,
		nowFunc:                    time.Now,
		logPrefix:                  opts.LogPrefix,
		doneCh:                     make(chan struct{}),
	}
	return u
}

func (u *Updater) Start(ctx context.Context) {
	if u.refreshPeriod <= 0 {
		return
	}

	go u.startRefreshLoop(ctx)
}

func (u *Updater) Stop(shutdownCtx context.Context) {
	u.logger().Debug(fmt.Sprintf("%s start to stop...", u.logPrefix))

	go func() {
		u.lockForCred.Lock()
		defer u.lockForCred.Unlock()
		if u.stopped {
			return
		}
		u.stopped = true
		close(u.doneCh)
	}()

	select {
	case <-shutdownCtx.Done():
	case <-u.doneCh:
	}
}

func (u *Updater) startRefreshLoop(ctx context.Context) {
	ticket := time.NewTicker(u.refreshPeriod)
	defer ticket.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-u.doneCh:
			break loop
		case <-ticket.C:
			u.refreshCredForLoop(ctx)
		}
	}
}

func (u *Updater) Credentials(ctx context.Context) (*Credentials, error) {
	if u.Expired() {
		if err := u.refreshCred(ctx); err != nil {
			return nil, err
		}
	}

	cred := u.getCred().DeepCopy()
	return cred, nil
}

func (u *Updater) refreshCredForLoop(ctx context.Context) {
	exp := u.expiration()

	if !u.expired(u.expiryWindowForRefreshLoop) {
		return
	}

	u.logger().Debug(fmt.Sprintf("%s start refresh credentials, current expiration: %s",
		u.logPrefix, exp.Format("2006-01-02T15:04:05Z")))

	maxRetry := 5
	for i := 0; i < maxRetry; i++ {
		err := u.refreshCred(ctx)
		if err == nil {
			return
		}
		if IsNotEnableError(err) {
			return
		}
		if i < maxRetry-1 {
			time.Sleep(time.Second * time.Duration(i))
		}
	}
}

func (u *Updater) refreshCred(ctx context.Context) error {
	cred, err := u.getCredentials(ctx)
	if err != nil {
		if IsNotEnableError(err) {
			return err
		}
		u.logger().Error(err, fmt.Sprintf("%s refresh credentials failed: %s", u.logPrefix, err))
		return err
	}
	u.logger().Debug(fmt.Sprintf("%s refreshed credentials, expiration: %s",
		u.logPrefix, cred.Expiration.Format("2006-01-02T15:04:05Z")))

	u.setCred(cred)
	return nil
}

func (u *Updater) setCred(cred *Credentials) {
	u.lockForCred.Lock()
	defer u.lockForCred.Unlock()

	newCred := cred.DeepCopy()
	newCred.Expiration = newCred.Expiration.Round(0)
	if u.expiryWindow > 0 {
		newCred.Expiration = newCred.Expiration.Add(-u.expiryWindow)
	}
	u.cred = newCred
}

func (u *Updater) getCred() *Credentials {
	u.lockForCred.RLock()
	defer u.lockForCred.RUnlock()

	return u.cred
}

func (u *Updater) Expired() bool {
	return u.expired(0)
}

func (u *Updater) expired(expiryDelta time.Duration) bool {
	exp := u.expiration()
	if expiryDelta > 0 {
		exp = exp.Add(-expiryDelta)
	}

	return exp.Before(u.now())
}

func (u *Updater) expiration() time.Time {
	cred := u.getCred()

	if cred == nil {
		return time.Time{}
	}

	return cred.Expiration.Round(0)
}

func (u *Updater) now() time.Time {
	if u.nowFunc == nil {
		return time.Now()
	}
	return u.nowFunc()
}

func (u *Updater) logger() Logger {
	if u.Logger != nil {
		return u.Logger
	}
	return defaultLog
}
