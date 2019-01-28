FROM tensorflow/tensorflow:1.11.0-py3

# keras config and resnet50 weights
RUN mkdir -p /root/.keras/models
ADD .keras.json /root/.keras
ADD ./weights/resnet50_weights_tf_dim_ordering_tf_kernels.h5 /root/.keras/models

# app 
ADD main.py /app/main.py
RUN chmod +x /app/main.py
ADD requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt

# manifext and engine config
ADD manifest.json /var/manifest.json
ADD ./dist/engine /app/engine
ENV VERITONE_WEBHOOK_READY="http://0.0.0.0:8080/readyz"
ENV VERITONE_WEBHOOK_PROCESS="http://0.0.0.0:8080/process"
#ENTRYPOINT [ "/app/engine", "/app/main.py" ]
EXPOSE 8080
ENTRYPOINT [ "/app/main.py" ]
