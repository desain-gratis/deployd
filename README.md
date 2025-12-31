
ini pasti dideploy di cluster mb1 mb2 mb3 biar murah. beda port. beda dragonboatconfig. 

yg pasti mycontent attachemtn untuk upload by commit id langsung (tanpa entity lainnya)

kemudian register availability. bahwa itu commit selesai upload
 - berdasarkan branch.
 branch name -> refs 



di github reponya ada DEPLOYD_CLIENT_ID, DEPLOYD_CLIENT_SECRET
-> admin dashboard untuk buat baru dan get secret
-> endpoint 1 untuk dapetin AUTHORIZATION TOKEN
-> endpoint 2 untuk upload2
-> endpoint 3 untuk update metadata for easy querying (commit ref / branch name / id / latest upload time / upload metadata / github actor or other CI metadata)
-> expose api for get release data
-> deploy 
     -> download archive .tgz based on cluster / node OS & architecture
     -> un-bundle archive to /tmp
     -> match with systemd / install it according to OS systemd config
        - copy specified binary to the /usr/lib /.... with new versioning
        - copy specified config directory to /etc/<my service> (maybe with config versioning as well)
        - update binary link & config link
        - reload / restart systemd
        - (optional) monitor deploymen / maybe run tests suite in testing namespace in production environment (non distributed;only local)
        - prompt user to continue
