# This version confirmed to work for both GitHub and AKS
FROM mcr.microsoft.com/windows/servercore:1809 as servercore

# Nanoserver is missing exactly one DLL that Go needs. The servercore image has this DLL, but
# is nearly 2GB. This has the potential to be somewhat fragile, but it is the only way I can
# find to keep the container image size small. To keep from doing this rather expensive
# multistage build every time, we build a small base image and push it to our repo
FROM mcr.microsoft.com/windows/nanoserver:1809

COPY --from=servercore /windows/system32/netapi32.dll /windows/system32/netapi32.dll

# certoc is required to add new certificates to the root store.
COPY --from=servercore /windows/system32/certoc.exe /windows/system32/certoc.exe

COPY setup-certificates.cmd /setup-certificates.cmd
