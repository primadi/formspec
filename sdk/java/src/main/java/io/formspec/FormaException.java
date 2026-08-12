package io.formspec;

/** Transport failure or sidecar-reported error. */
public final class FormaException extends RuntimeException {
    public FormaException(String message) {
        super(message);
    }

    public FormaException(String message, Throwable cause) {
        super(message, cause);
    }
}
