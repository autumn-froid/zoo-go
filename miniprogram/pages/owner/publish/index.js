const {request, mediaUrl} = require('../../../utils/api')
const {tapFeedback} = require('../../../utils/format')

Page({
    data: {
        loading: true,
        submitting: false,
        pets: [],
        petId: '',
        serviceOptions: [],
        dayOptions: [],
        timeOptions: [],
        deadlineOptions: [],
        communityOptions: [],
        serviceDate: '',
        serviceDateText: '',
        timeWindow: '',
        deadlineMinutes: 60,
        community: '',
        locationName: '',
        latitude: 0,
        longitude: 0,
        addressDetail: '',
        mode: 'bid',
        budget: 60,
        note: '',
        safetyConfirmed: false,
        cameraConfirmed: false,
        mediaConsent: false,
        agreementAccepted: false,
        agreement: null,
        agreementVersion: '',
        identityVerified: false,
    },

    async onLoad() {
        try {
            const [meta, home, agreement] = await Promise.all([
                request('/v1/meta'),
                request('/v1/owner/home'),
                request('/v1/agreements/current?role=owner'),
            ])
            const pets = home.pets || []
            const day = meta.days[Math.min(2, meta.days.length - 1)] || meta.days[0]
            const defaultServices = new Set(meta.serviceTypes.slice(0, 2))
            const savedAddress = home.address || null
            this.setData({
                loading: false,
                pets: pets.map((pet) => ({...pet, image: mediaUrl(pet.image)})),
                petId: pets[0] ? pets[0].id : '',
                serviceOptions: meta.serviceTypes.map((value) => ({value, selected: defaultServices.has(value)})),
                dayOptions: meta.days.map((item) => ({...item, selected: item.value === day.value})),
                timeOptions: meta.timeWindows.map((value, index) => ({value, selected: index === 0})),
                deadlineOptions: meta.deadlines.map((item) => ({...item, selected: item.minutes === 60})),
                serviceDate: day.value,
                serviceDateText: day.label,
                timeWindow: meta.timeWindows[0],
                community: savedAddress && meta.communities.includes(savedAddress.community)
                    ? savedAddress.community
                    : meta.communities[0],
                communityOptions: meta.communities.map((value) => ({
                    value,
                    selected: value === (savedAddress && meta.communities.includes(savedAddress.community) ? savedAddress.community : meta.communities[0]),
                })),
                addressDetail: savedAddress ? savedAddress.detail : '',
                locationName: savedAddress ? savedAddress.locationName || '' : '',
                latitude: savedAddress ? savedAddress.latitude || 0 : 0,
                longitude: savedAddress ? savedAddress.longitude || 0 : 0,
                agreement,
                agreementVersion: agreement.version,
                identityVerified: Boolean(home.profile && home.profile.verified),
            })
            this.ready = true
            wx.setNavigationBarTitle({title: '发布喂宠任务'})
        } catch (error) {
            this.setData({loading: false})
            wx.showToast({title: error.message, icon: 'none'})
        }
    },

    onShow() {
        if (this.ready) {
            this.refreshPets()
        }
    },

    async refreshPets() {
        try {
            const home = await request('/v1/owner/home')
            const pets = home.pets || []
            const selected = pets.some((pet) => pet.id === this.data.petId) ? this.data.petId : (pets[0] ? pets[0].id : '')
            const address = home.address
            this.setData({
                pets: pets.map((pet) => ({...pet, image: mediaUrl(pet.image)})),
                petId: selected,
                addressDetail: this.data.addressDetail || (address ? address.detail : ''),
                identityVerified: Boolean(home.profile && home.profile.verified),
            })
        } catch (error) {
            wx.showToast({title: error.message, icon: 'none'})
        }
    },

    selectPet(event) {
        this.setData({petId: event.currentTarget.dataset.value})
    },

    toggleService(event) {
        const value = event.currentTarget.dataset.value
        const serviceOptions = this.data.serviceOptions.map((item) => item.value === value ? {...item, selected: !item.selected} : item)
        this.setData({serviceOptions})
    },

    selectDay(event) {
        const value = event.currentTarget.dataset.value
        const option = this.data.dayOptions.find((item) => item.value === value)
        this.setData({
            serviceDate: value,
            serviceDateText: option.label,
            dayOptions: this.data.dayOptions.map((item) => ({...item, selected: item.value === value})),
        })
    },

    selectTime(event) {
        const value = event.currentTarget.dataset.value
        this.setData({
            timeWindow: value,
            timeOptions: this.data.timeOptions.map((item) => ({...item, selected: item.value === value})),
        })
    },

    selectDeadline(event) {
        const value = Number(event.currentTarget.dataset.value)
        this.setData({
            deadlineMinutes: value,
            deadlineOptions: this.data.deadlineOptions.map((item) => ({...item, selected: item.minutes === value})),
        })
    },

    selectCommunity(event) {
        const value = event.currentTarget.dataset.value
        this.setData({
            community: value,
            communityOptions: this.data.communityOptions.map((item) => ({...item, selected: item.value === value})),
        })
    },

    selectMode(event) {
        tapFeedback()
        this.setData({mode: event.currentTarget.dataset.value})
    },

    changeBudget(event) {
        const delta = Number(event.currentTarget.dataset.delta)
        const next = Math.max(30, Math.min(200, this.data.budget + delta))
        this.setData({budget: next})
    },

    onNoteInput(event) {
        this.setData({note: event.detail.value})
    },

    onAddressInput(event) {
        this.setData({addressDetail: event.detail.value})
    },

    chooseServiceLocation() {
        wx.chooseLocation({
            success: (result) => {
                const locationName = (result.name || result.address || '已选择位置').trim()
                this.setData({
                    locationName,
                    latitude: result.latitude,
                    longitude: result.longitude,
                    addressDetail: this.data.addressDetail || result.name || '',
                })
            },
            fail: (error) => {
                if ((error.errMsg || '').includes('cancel')) {
                    return
                }
                wx.showToast({title: '选择位置失败，请检查定位权限', icon: 'none'})
            },
        })
    },

    goAddPet() {
        wx.navigateTo({url: '/pages/owner/pet-edit/index'})
    },

    toggleSafety(event) {
        this.setData({safetyConfirmed: event.detail.value})
    },

    toggleAgreement() {
        this.setData({agreementAccepted: !this.data.agreementAccepted})
    },

    toggleConsent(event) {
        const field = event.currentTarget.dataset.field
        if (field) {
            this.setData({[field]: !this.data[field]})
        }
    },

    showAgreement() {
        wx.navigateTo({url: '/pages/agreement/index?role=owner'})
    },

    goIdentity() {
        wx.navigateTo({url: '/pages/identity/index'})
    },

    async submit() {
        if (this.data.submitting) {
            return
        }
        const services = this.data.serviceOptions.filter((item) => item.selected).map((item) => item.value)
        if (!this.data.petId) {
            wx.showToast({title: '请先添加一只宠物', icon: 'none'})
            return
        }
        if (!this.data.identityVerified) {
            wx.showModal({
                title: '请先完成实名认证',
                content: '发布任务前需要核验交易双方真实身份。实名资料审核通过后即可发布。',
                confirmText: '去认证',
                success: (result) => {
                    if (result.confirm) {
                        this.goIdentity()
                    }
                },
            })
            return
        }
        if (!services.length) {
            wx.showToast({title: '请至少选择一项服务', icon: 'none'})
            return
        }
        if (!this.data.safetyConfirmed) {
            wx.showToast({title: '请确认宠物安全情况', icon: 'none'})
            return
        }
        if (!this.data.cameraConfirmed) {
            wx.showToast({title: '请如实披露室内摄像头与录音设备', icon: 'none'})
            return
        }
        if (!this.data.mediaConsent) {
            wx.showToast({title: '请确认履约照片使用范围', icon: 'none'})
            return
        }
        if (!this.data.agreementAccepted || !this.data.agreementVersion) {
            wx.showToast({title: '请先阅读并同意服务协议', icon: 'none'})
            return
        }
        if (!this.data.addressDetail.trim()) {
            wx.showToast({title: '请填写详细门牌地址', icon: 'none'})
            return
        }
        if (!this.data.latitude || !this.data.longitude) {
            wx.showToast({title: '请在地图中选择服务地址位置', icon: 'none'})
            return
        }
        this.setData({submitting: true})
        try {
            const task = await request('/v1/tasks', {
                method: 'POST',
                data: {
                    petId: this.data.petId,
                    services,
                    serviceDate: this.data.serviceDate,
                    serviceDateText: this.data.serviceDateText,
                    timeWindow: this.data.timeWindow,
                    mode: this.data.mode,
                    budgetFen: this.data.budget * 100,
                    bidDurationMinutes: this.data.deadlineMinutes,
                    community: this.data.community,
                    locationName: this.data.locationName,
                    latitude: this.data.latitude,
                    longitude: this.data.longitude,
                    addressDetail: this.data.addressDetail,
                    note: this.data.note,
                    safetyConfirmed: true,
                    agreementVersion: this.data.agreementVersion,
                    agreementItems: [
                        'owner_health_truth',
                        'owner_camera_disclosure',
                        'owner_media_consent',
                        'service_agreement',
                    ],
                    agreementAccepted: true,
                },
            })
            wx.showToast({title: '发布成功', icon: 'success'})
            setTimeout(() => {
                wx.redirectTo({url: `/pages/owner/task/index?id=${task.id}`})
            }, 450)
        } catch (error) {
            wx.showToast({title: error.message, icon: 'none'})
            this.setData({submitting: false})
        }
    },
})
