# Ensure conda is available
eval "$(conda shell.bash hook)"

# Create and activate environment
conda create --yes --name pb condaforge::go=1.24.4
conda activate pb