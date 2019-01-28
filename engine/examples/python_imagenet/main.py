#!/usr/bin/env python
import sys
import traceback
import numpy as np
import logging
from keras.applications.resnet50 import ResNet50
from keras.preprocessing import image
from keras.applications.resnet50 import preprocess_input, decode_predictions
from io import BytesIO
from flask import Flask, jsonify, request
from gevent import wsgi

# adjust server logging
log = logging.getLogger('werkzeug')
log.setLevel(logging.DEBUG)

# load the pre-trained model for imagenet
model = ResNet50(weights='imagenet')

# flask and server
app = Flask(__name__)
server = None

@app.route('/')
def index():
    return readyz()

@app.route('/readyz', methods=['GET'])
def readyz():
    return 'OK', 200

@app.route('/predict', methods=['POST'])
def predict():
    try:
        # load the chunk and save it to a buffer
        file = request.files['chunk']
        chunk = BytesIO()
        file.save(chunk)

        # pre process the image features
        img = image.load_img(chunk, target_size=(224, 224))
        x = image.img_to_array(img)
        x = np.expand_dims(x, axis=0)
        x = preprocess_input(x)

        # make the prediction
        preds = model.predict(x)
        # decode predictions and get the most probable class and confidence
        decoded = decode_predictions(preds, top=1)[0]
        image_class = decoded[0][1]
        image_confidence = float(decoded[0][2])
 
        return jsonify({'success': True, 'class': image_class, 'confidence': image_confidence})
    except:
        tb = traceback.format_exc()        
        app.logger.error('error predicting %s', str(tb))
        return str(tb), 500


@app.route('/shutdown', methods=['DELETE'])
def shutdown():
    app.logger.info('shuting the server down')
    server.stop()
    app.logger.info('server terminated')
    return 'OK', 200    

if __name__ == '__main__':
    port = 8080
    server = wsgi.WSGIServer(('0.0.0.0', int(port)), app)
    server.serve_forever()
