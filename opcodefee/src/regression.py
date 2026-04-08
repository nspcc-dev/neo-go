import numpy as np
from sklearn.metrics import r2_score
from sklearn.model_selection import train_test_split

FACTOR = 1000


def myfunc_w(X, y, power=1.0, eps=1e-6):
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.25, random_state=42,
    )

    X_train_mat = X_train.to_numpy(dtype=float)
    X_test_mat = X_test.to_numpy(dtype=float)
    ys = y_train.to_numpy(dtype=float)

    max_j = np.max(X_train_mat, axis=0)

    normalized = X_train_mat / max_j
    sum_norm = np.sum(normalized, axis=1) + eps

    weights = 1.0 / (sum_norm ** power)

    A = np.hstack([X_train_mat, np.ones((X_train_mat.shape[0], 1))])

    W = weights[:, None]
    A_w = W * A
    y_w = W.ravel() * ys

    beta, *_ = np.linalg.lstsq(A_w, y_w, rcond=None)

    A_test = np.hstack([X_test_mat, np.ones((X_test_mat.shape[0], 1))])
    y_pred = A_test @ beta

    r2 = float(r2_score(y_test.to_numpy(dtype=float), y_pred))

    return beta, r2


def run_myfunc_w(df, target="ns", power=1.0, drop_columns=None, handler=None, factor=FACTOR):
    if handler is not None:
        df = handler(df)

    if drop_columns is None:
        drop_columns = []

    X = df.drop(columns=[target] + drop_columns)
    y = df[target]

    beta, r2 = myfunc_w(X, y, power=power)
    return np.int64(np.round(beta * factor)), r2


def run_constant_price(df, target="ns", factor=FACTOR):
    return np.int64(np.round(df[target].item() * factor))
