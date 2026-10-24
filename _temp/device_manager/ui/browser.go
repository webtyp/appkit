//go:build browser

package ui

import (
	devicemanager "github.com/veltylabs/device_manager"
	"webtyp.com/fmt"
	"webtyp.com/icons/laptop"
	"webtyp.com/icons/location"
	"webtyp.com/icons/network"
	"webtyp.com/lang"
	"webtyp.com/layout/crudview"
	"webtyp.com/layout/platformd"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

// Campos select cuyas opciones provienen de otra lista de este módulo.
const (
	fieldZoneID   = "zone_id"
	fieldDeviceID = "device_id"
)

// Textos de ayuda que el chasis muestra en el botón «?» de cada pantalla. Las
// claves están en inglés; la traducción vive en lang.json.
const (
	descDevices    lang.Text = "Each computer, printer or device of the clinic, with the zone where it is. Its network interfaces are added in Network interfaces."
	descZones      lang.Text = "Zones group the devices by place (for example, reception or box 1). Create the zones first; each device then picks one."
	descInterfaces lang.Text = "The network connections of each device: its MAC and IP address. Staff log in from the devices registered here."
)

// Browser construye la pantalla de equipos. El selector de zona se llena desde list_zones
// al ACTIVARSE la pantalla (nunca al instanciarse: nada llama al servidor hasta que el
// usuario abre la pantalla).
//
// Ejemplo: mod, err := ui.Browser(caller, ids, tenantID)
func Browser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error) {
	v, err := crudview.New(crudview.Config{
		ParentID:  ID,
		Presenter: devicemanager.NewView(caller),
		IDs:       ids,
	})
	if err != nil {
		return nil, err
	}
	activate := onFirstActivation(v, func() {
		fillOptions(v, fieldZoneID, devicemanager.NewZoneView(caller))
	})
	return platformd.Describe(platformd.NewUIModule(ID, Label, laptop.Ref, v, activate), descDevices), nil
}

// ZonesBrowser construye la pantalla de zonas.
//
// Ejemplo: mod, err := ui.ZonesBrowser(caller, ids, tenantID)
func ZonesBrowser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error) {
	v, err := crudview.New(crudview.Config{
		ParentID:  ZonesID,
		Presenter: devicemanager.NewZoneView(caller),
		IDs:       ids,
	})
	if err != nil {
		return nil, err
	}
	return platformd.Describe(platformd.NewUIModule(ZonesID, ZonesLabel, location.Ref, v), descZones), nil
}

// InterfacesBrowser construye la pantalla de interfaces de red. Su selector de equipo se
// completa desde list_devices al activarse.
//
// Ejemplo: mod, err := ui.InterfacesBrowser(caller, ids, tenantID)
func InterfacesBrowser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error) {
	v, err := crudview.New(crudview.Config{
		ParentID:  InterfacesID,
		Presenter: devicemanager.NewNetworkInterfaceView(caller),
		IDs:       ids,
	})
	if err != nil {
		return nil, err
	}
	activate := onFirstActivation(v, func() {
		fillOptions(v, fieldDeviceID, devicemanager.NewView(caller))
	})
	return platformd.Describe(platformd.NewUIModule(InterfacesID, InterfacesLabel, network.Ref, v, activate), descInterfaces), nil
}

// onFirstActivation devuelve el hook onActivate de platformd: la carga se ejecuta una sola
// vez, la primera vez que la pantalla se activa; la carga perezosa de la lista propia de
// crudview se ejecuta cada vez (platformd llama a onActivate EN LUGAR del Activate de la vista).
func onFirstActivation(v *crudview.CrudView, load func()) func() {
	loaded := false
	return func() {
		if !loaded {
			loaded = true
			load()
		}
		v.Activate()
	}
}

// fillOptions carga source y asigna sus filas (ID → Label) como las opciones del campo
// select de v. Una carga fallida deja el select vacío; la lista misma informa sus propios
// errores de carga.
func fillOptions(v *crudview.CrudView, field string, source view.Presenter) {
	source.Reload(func(err error) {
		if err != nil {
			return
		}
		items := source.Items()
		opts := make([]fmt.KeyValue, 0, len(items))
		for _, it := range items {
			opts = append(opts, fmt.KeyValue{Key: it.ID, Value: it.Label})
		}
		v.SetOptions(field, opts...)
	})
}
