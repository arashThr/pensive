FROM continuumio/miniconda3:latest

WORKDIR /app

RUN apt-get update && \
    apt-get -y install gcc g++ make

# Copy environment file
COPY environment.yml .

# Create conda environment
RUN conda env create -f environment.yml

# Make RUN commands use the new environment:
RUN echo "conda activate kokoro" >> ~/.bashrc
SHELL ["/bin/bash", "--login", "-c"]

# Demonstrate the environment is activated:
RUN echo "Make sure flask is installed:"
RUN python -c "import flask"

# Create uploads directory
RUN mkdir -p /app/uploads/podcasts

# Expose port
EXPOSE 5000

# The code to run when container is started:
COPY app.py entrypoint.sh ./
ENTRYPOINT ["bash", "./entrypoint.sh"]
