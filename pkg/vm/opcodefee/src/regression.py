import numpy as np

FACTOR = 1000


def weighted_lstsq(X, y, power=1.0, eps=1e-6):
    X_mat = X.to_numpy(dtype=float)
    ys = y.to_numpy(dtype=float)

    max_j = np.max(X_mat, axis=0)

    normalized = X_mat / max_j
    sum_norm = np.sum(normalized, axis=1) + eps

    weights = 1.0 / (sum_norm ** power)

    A = np.hstack([X_mat, np.ones((X_mat.shape[0], 1))])

    W = weights[:, None]
    A_w = W * A
    y_w = W.ravel() * ys

    beta, *_ = np.linalg.lstsq(A_w, y_w, rcond=None)

    return beta


def fit_weights(df, target="nops", power=1.0, drop_columns=None, handler=None, factor=FACTOR):
    if handler is not None:
        df = handler(df)

    if drop_columns is None:
        drop_columns = []

    X = df.drop(columns=[target] + drop_columns)
    y = df[target]

    beta = weighted_lstsq(X, y, power=power)
    return dict(zip([*X.columns, "intercept"], np.int64(np.round(beta * factor)).tolist()))


def print_weights(name, weights):
    print(f"{name}: " + ", ".join(f"{k}={v}" for k, v in weights.items()))
