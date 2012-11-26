"""Easier handling of exceptions with error numbers."""

import __builtin__
import abc
import errno
import re
import sys


# Get a list of all named errors.
ERRORS = dict((name, value) for name, value in vars(errno).iteritems()
              if re.match(r'^E[A-Z]+$', name))


class Error(__builtin__.EnvironmentError):

    """
    An abstract base class which can be used to check for errors with errno.

    Example:

        >>> read_fd, write_fd = os.pipe()
        >>> try:
        ...     os.read(write_fd, 256)
        ... except Error.EBADF, exc:
        ...     print "EBADF raised: %r" % exc
        EBADF raised: OSError(9, 'Bad file descriptor')

    Other errors, of the same type but with a different errno, will not be
    caught:

        >>> read_fd, write_fd = os.pipe()
        >>> os.close(read_fd)
        >>> try:
        ...     os.write(write_fd, "Hello!\\n")
        ... except Error.EBADF, exc:
        ...     print "EBADF raised: %r" % exc
        Traceback (most recent call last):
        ...
        OSError: [Errno 32] Broken pipe

    You can catch several errors using the standard Python syntax:

        >>> try:
        ...     os.write(write_fd, "Hello!\\n")
        ... except (Error.EBADF, Error.EPIPE):
        ...     print "Problem writing to pipe"
        Problem writing to pipe

    And catch different errors on different lines:

        >>> try:
        ...     os.write(write_fd, "Hello!\\n")
        ... except Error.EBADF:
        ...     print "Pipe was closed at this end"
        ... except Error.EPIPE:
        ...     print "Pipe was closed at the other end"
        Pipe was closed at the other end
    """

    match_errno = None

    class __metaclass__(abc.ABCMeta):
        def __getattr__(cls, error_name):
            if cls.match_errno is None and error_name in ERRORS:
                return cls.matcher(error_name, ERRORS[error_name])
            raise AttributeError(error_name)

    def __init__(self):
        raise TypeError("Cannot create teena.Error instances")

    @classmethod
    def __subclasshook__(cls, exc_type):
        exc_type, exc_info, traceback = sys.exc_info()
        exc_errno = getattr(exc_info, 'errno', None)
        if cls.match_errno is None:
            return exc_errno is not None
        elif exc_errno is not None:
            return exc_errno == cls.match_errno
        return False

    @classmethod
    def matcher(cls, error_name, error_number):
        # Return a dynamically-created subclass of the current class with the
        # `match_errno` attribute set.
        return type(cls.__name__ + "." + error_name,
                    (cls,),
                    {'match_errno': error_number,
                     '__module__': cls.__module__})
