
name = "SIMPLEX"
version = "1.0"

class types:
    request = "req"
    reply = "rep"

class handshake:
    accept = "+OK"

def request(payload, method, id=None, more=None, ext=None):
    msg = dict(
        type=types.request,
        method=method,
        payload=payload,
    )
    if id is not None:
        msg['id'] = id
    if more is True:
        msg['more'] = True
    if ext is not None:
        msg['ext'] = ext
    return msg

def reply(id, payload, more=None, ext=None):
    msg = dict(
        type=types.reply,
        id=id,
        payload=payload,
    )
    if more is True:
        msg['more'] = True
    if ext is not None:
        msg['ext'] = ext
    return msg

def error(id, code, message, data=None, ext=None):
    msg = dict(
        type=types.reply,
        id=id,
        error=dict(
            code=code,
            message=message,
        ),
    )
    if data is not None:
        msg['error']['data'] = data
    if ext is not None:
        msg['ext'] = ext
    return msg
