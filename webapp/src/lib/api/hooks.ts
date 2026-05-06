import { useEffect, useEffectEvent, useState } from 'react';
import { ApiError, getDashboardSummary, getHealth, unwrapEnvelope } from './client';
import type { DashboardSummary, HealthResponse } from './types';

type AsyncState<T> = {
  data: T | null;
  error: ApiError | null;
  loading: boolean;
};

function useApiQuery<T>(queryFn: (signal: AbortSignal) => Promise<T>) {
  const [state, setState] = useState<AsyncState<T>>({
    data: null,
    error: null,
    loading: true,
  });
  const runQuery = useEffectEvent(queryFn);

  useEffect(() => {
    const controller = new AbortController();

    setState((current) => ({ ...current, loading: true, error: null }));

    runQuery(controller.signal)
      .then((data) => {
        setState({ data, error: null, loading: false });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }

        setState({
          data: null,
          error: error instanceof ApiError ? error : new ApiError(0, '请求失败'),
          loading: false,
        });
      });

    return () => {
      controller.abort();
    };
  }, [runQuery]);

  return state;
}

export function useHealthQuery() {
  return useApiQuery<HealthResponse>((signal) =>
    getHealth({ signal }).then((payload) => unwrapEnvelope(payload)),
  );
}

export function useDashboardSummaryQuery() {
  return useApiQuery<DashboardSummary>((signal) =>
    getDashboardSummary({ signal }).then((payload) => unwrapEnvelope(payload)),
  );
}
