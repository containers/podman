from flask import Flask
import os
app = Flask(__name__)

@app.route('/')
def hello():
    passthru = "ERROR: Could not get $ENV_PASSTHRU envariable"
    try:
        passthru = os.getenv("ENV_PASSTHRU")
    except Exception as e:
        passthru = passthru + ": " + str(e)
    return "Podman rulez!--" + passthru + "--!"

if __name__ == '__main__':
    app.run(host='0.0.0.0')
